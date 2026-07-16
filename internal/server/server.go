// Package server hosts the embedded UI on a loopback socket. The webview
// window (settings or picker) navigates to it; all state flows through the
// JSON API below.
package server

import (
	"embed"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"switchboard/internal/companion"
	"switchboard/internal/config"
	"switchboard/internal/profiles"
	"switchboard/internal/route"
	"switchboard/internal/winutil"
)

//go:embed ui
var uiFS embed.FS

// Server carries the picker context (empty in settings mode) and a callback
// used to close the webview window after the user picks a profile.
type Server struct {
	PickURL    string
	PickSource string
	OnPicked   func()
}

// Start serves on an ephemeral loopback port and returns the base URL.
// SWITCHBOARD_ADDR pins the address (e.g. "127.0.0.1:7777") for debugging.
func (s *Server) Start() (string, error) {
	addr := os.Getenv("SWITCHBOARD_ADDR")
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page("ui/index.html"))
	mux.HandleFunc("GET /pick", s.page("ui/pick.html"))
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("POST /api/config", s.handleSaveConfig)
	mux.HandleFunc("POST /api/test", s.handleTest)
	mux.HandleFunc("POST /api/launch", s.handleLaunch)
	mux.HandleFunc("POST /api/install", s.handleInstall)
	mux.HandleFunc("POST /api/uninstall", s.handleUninstall)
	mux.HandleFunc("POST /api/opensettings", s.handleOpenSettings)
	mux.HandleFunc("POST /api/autostart", s.handleAutostart)
	go http.Serve(ln, mux)
	return "http://" + ln.Addr().String(), nil
}

func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := uiFS.ReadFile(name)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}

type stateResp struct {
	Exe        string             `json:"exe"`
	ConfigPath string             `json:"configPath"`
	Installed  bool               `json:"installed"`
	IsDefault  bool               `json:"isDefault"`
	Autostart  bool               `json:"autostart"`
	Config     *config.Config     `json:"config"`
	Discovered []profiles.Profile `json:"discovered"`
	Log        []route.Entry      `json:"log"`
	PickURL    string             `json:"pickUrl,omitempty"`
	PickSource string             `json:"pickSource,omitempty"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	exe, _ := os.Executable()
	writeJSON(w, stateResp{
		Exe:        exe,
		ConfigPath: config.Path(),
		Installed:  winutil.Installed(),
		IsDefault:  winutil.IsDefaultBrowser(),
		Autostart:  winutil.AutostartEnabled(),
		Config:     cfg,
		Discovered: profiles.Discover(),
		Log:        route.Tail(50),
		PickURL:    s.PickURL,
		PickSource: s.PickSource,
	})
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	for _, rl := range cfg.Rules {
		if _, ok := cfg.Profiles[rl.Profile]; !ok {
			http.Error(w, "rule references unknown profile: "+rl.Profile, 400)
			return
		}
	}
	if err := config.Save(&cfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	var req struct{ URL, Source string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	profile, via := route.Decide(cfg, req.Source, req.URL)
	writeJSON(w, map[string]string{"profile": profile, "via": via})
}

// handleLaunch opens the picker's URL in the chosen profile, optionally
// persisting a rule for the URL's host, then closes the picker window.
func (s *Server) handleLaunch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile  string `json:"profile"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := route.Launch(cfg, req.Profile, s.PickURL); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	via := "picker"
	if req.Remember {
		if host := urlHost(s.PickURL); host != "" {
			cfg.Rules = append(cfg.Rules, config.Rule{URL: "*://" + host + "/*", Profile: req.Profile})
			config.Save(cfg)
			via = "picker (rule saved)"
		}
	}
	route.Log(s.PickSource, s.PickURL, req.Profile, via)
	writeJSON(w, map[string]bool{"ok": true})
	if s.OnPicked != nil {
		go s.OnPicked()
	}
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	exe, err := os.Executable()
	if err == nil {
		err = winutil.Install(exe)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleOpenSettings(w http.ResponseWriter, r *http.Request) {
	winutil.OpenDefaultAppsSettings()
	writeJSON(w, map[string]bool{"ok": true})
}

// handleAutostart toggles the login Run entry; enabling also starts the tray
// right away so the icon appears without a reboot.
func (s *Server) handleAutostart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	exe, err := os.Executable()
	if err == nil {
		err = winutil.SetAutostart(exe, req.Enabled)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if req.Enabled {
		exec.Command(exe, "tray").Start()
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleUninstall(w http.ResponseWriter, r *http.Request) {
	if err := winutil.Uninstall(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := companion.Unregister(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func urlHost(url string) string {
	rest := url
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.IndexByte(rest, '@'); i >= 0 {
		rest = rest[i+1:]
	}
	if i := strings.IndexByte(rest, ':'); i >= 0 {
		rest = rest[:i]
	}
	return strings.ToLower(rest)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
