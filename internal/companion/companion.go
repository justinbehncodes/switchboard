// Package companion implements the browser-extension side channel: links
// clicked *inside* a browser never reach the default-browser handler, so a
// small extension asks us over Chrome native messaging whether the URL
// belongs in a different profile.
package companion

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"switchboard/internal/config"
	"switchboard/internal/profiles"
	"switchboard/internal/rules"
)

//go:embed ext
var extFS embed.FS

const (
	// HostName is the native messaging host identifier the extension calls.
	HostName = "com.switchboard.router"
	// ExtensionID is pinned by the "key" field in ext/manifest.json;
	// regenerate both together with tools/genkey.
	ExtensionID = "iokjnancgjlnlmppeacpfoiacmoddbli"
)

// Deploy writes the unpacked extension to <baseDir>\extension and the native
// messaging host manifest to <baseDir>\nm-manifest.json.
func Deploy(baseDir, exe string) error {
	extDir := filepath.Join(baseDir, "extension")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		return err
	}
	err := fs.WalkDir(extFS, "ext", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := extFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(extDir, filepath.Base(path)), data, 0o644)
	})
	if err != nil {
		return err
	}

	manifest := map[string]any{
		"name":            HostName,
		"description":     "Switchboard link router",
		"path":            exe,
		"type":            "stdio",
		"allowed_origins": []string{"chrome-extension://" + ExtensionID + "/"},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ManifestPath(baseDir), data, 0o644)
}

func ManifestPath(baseDir string) string {
	return filepath.Join(baseDir, "nm-manifest.json")
}

var hostRoots = []string{
	`Software\Google\Chrome\NativeMessagingHosts\`,
	`Software\Microsoft\Edge\NativeMessagingHosts\`,
}

// Register points Chrome's and Edge's native messaging host lookups at the
// manifest. Per-user, reversible.
func Register(baseDir string) error {
	for _, root := range hostRoots {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, root+HostName, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = k.SetStringValue("", ManifestPath(baseDir))
		k.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func Unregister() error {
	for _, root := range hostRoots {
		if err := registry.DeleteKey(registry.CURRENT_USER, root+HostName); err != nil && err != registry.ErrNotExist {
			return err
		}
	}
	return nil
}

// Msg is what the extension sends: the link URL, the host of the page it was
// clicked on, which browser the extension runs in, and the profile's signed-in
// email (best effort) so we can tell which profile the click came from.
type Msg struct {
	URL     string `json:"url"`
	From    string `json:"from"`
	Browser string `json:"browser"`
	Email   string `json:"email"`
}

type Resp struct {
	Action  string `json:"action"` // "stay" | "grabbed"
	Profile string `json:"profile,omitempty"`
	Reason  string `json:"reason"`
}

// Decide is the pure decision: should this in-browser navigation be grabbed
// and reopened elsewhere? Source rules match the originating page's host
// (e.g. "chat.google.com") — exe-name rules simply never match here. The
// fallback never applies: for links already opening in a browser, staying put
// is always the right default.
func Decide(cfg *config.Config, discovered []profiles.Profile, msg Msg) Resp {
	idx, ok := rules.Match(cfg.Rules, msg.From, msg.URL)
	if !ok {
		return Resp{Action: "stay", Reason: "no rule matched"}
	}
	r := cfg.Rules[idx]
	target, exists := cfg.Profiles[r.Profile]
	if !exists {
		return Resp{Action: "stay", Reason: "rule points at unknown profile " + r.Profile}
	}
	if target.Browser == msg.Browser {
		if msg.Email == "" {
			// Same browser and we can't tell profiles apart: don't yank.
			return Resp{Action: "stay", Reason: "same browser, profile unknown"}
		}
		for _, d := range discovered {
			if d.Browser == msg.Browser && strings.EqualFold(d.Email, msg.Email) {
				if d.Dir == target.Dir {
					return Resp{Action: "stay", Reason: "already in profile " + r.Profile}
				}
				break
			}
		}
	}
	return Resp{Action: "grabbed", Profile: r.Profile, Reason: ruleDesc(r)}
}

func ruleDesc(r config.Rule) string {
	var parts []string
	if r.Source != "" {
		parts = append(parts, "source="+r.Source)
	}
	if r.URL != "" {
		parts = append(parts, "url="+r.URL)
	}
	return strings.Join(parts, " ")
}
