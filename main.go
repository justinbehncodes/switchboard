// Switchboard routes clicked links to the right browser profile.
//
// Modes:
//
//	switchboard.exe <url>      route a link (this is what Windows invokes)
//	switchboard.exe            settings window
//	switchboard.exe ui         settings window
//	switchboard.exe install    register as a browser (per-user, no admin)
//	switchboard.exe uninstall  remove the registration
//	switchboard.exe doctor     check profiles, rules, and registration
package main

import (
	"fmt"
	"os"
	"strings"

	webview2 "github.com/jchv/go-webview2"

	"switchboard/internal/companion"
	"switchboard/internal/config"
	"switchboard/internal/profiles"
	"switchboard/internal/route"
	"switchboard/internal/server"
	"switchboard/internal/tray"
	"switchboard/internal/winutil"
)

// version is stamped by the release build via -ldflags "-X main.version=…".
var version = "dev"

func main() {
	// Must be the first thing that runs: the app the link was clicked in is
	// only reliably still the foreground window at process start.
	source := winutil.ForegroundProcessName()

	args := os.Args[1:]
	if len(args) == 0 {
		openSettings()
		return
	}
	switch arg := args[0]; {
	case strings.HasPrefix(arg, "chrome-extension://"):
		// Invoked by Chrome/Edge as a native messaging host.
		companion.RunHost()
	case arg == "ui":
		openSettings()
	case arg == "tray":
		if err := tray.Run(); err != nil {
			winutil.MessageBox("Switchboard", "Tray failed: "+err.Error())
			os.Exit(1)
		}
	case arg == "install":
		winutil.AttachParentConsole()
		exe, err := os.Executable()
		must(err)
		must(winutil.Install(exe))
		if winutil.IsDefaultBrowser() {
			fmt.Println("Registered. Already the default browser.")
		} else {
			fmt.Println("Registered. Pick Switchboard as the default browser in the Settings page that just opened.")
			winutil.OpenDefaultAppsSettings()
		}
	case arg == "extension":
		// Unreleased: deploys the companion extension (routes links clicked
		// inside a browser) for loading unpacked. Off the default install
		// path until the extension ships through the browser stores.
		winutil.AttachParentConsole()
		exe, err := os.Executable()
		must(err)
		must(companion.Deploy(config.Dir(), exe))
		must(companion.Register(config.Dir()))
		fmt.Printf("Extension deployed to %s\\extension — load it via chrome://extensions (Developer mode > Load unpacked).\n", config.Dir())
	case arg == "uninstall":
		winutil.AttachParentConsole()
		must(winutil.Uninstall())
		must(companion.Unregister())
		fmt.Println("Unregistered.")
	case arg == "version":
		winutil.AttachParentConsole()
		fmt.Println("switchboard " + version)
	case arg == "doctor":
		winutil.AttachParentConsole()
		doctor()
	case arg == "test":
		winutil.AttachParentConsole()
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: switchboard test <url> [source.exe]")
			os.Exit(2)
		}
		testSource := ""
		if len(args) > 2 {
			testSource = args[2]
		}
		cfg, err := config.Load()
		must(err)
		profile, via := route.Decide(cfg, testSource, args[1])
		if profile == route.Ask {
			fmt.Printf("-> picker (%s)\n", via)
		} else {
			fmt.Printf("-> %s (%s)\n", profile, via)
		}
	case route.IsWebURL(arg):
		routeURL(source, arg)
	default:
		winutil.AttachParentConsole()
		fmt.Fprintf(os.Stderr, "unknown argument %q\nusage: switchboard [ui|install|uninstall|doctor|test <url> [source]|<url>]\n", arg)
		os.Exit(2)
	}
}

func routeURL(source, url string) {
	cfg, err := config.Load()
	if err != nil {
		winutil.MessageBox("Switchboard", "Config error: "+err.Error())
		os.Exit(1)
	}
	url = route.Unwrap(url)
	profile, via := route.Decide(cfg, source, url)
	if profile == route.Ask {
		openPicker(source, url)
		return
	}
	if err := route.Launch(cfg, profile, url); err != nil {
		winutil.MessageBox("Switchboard", "Launch failed: "+err.Error())
		os.Exit(1)
	}
	route.Log(source, url, profile, via)
}

func openSettings() {
	srv := &server.Server{}
	base, err := srv.Start()
	must(err)
	if !openWindow(base+"/", "Switchboard", 980, 760, nil) {
		winutil.MessageBox("Switchboard", "Could not create a window (WebView2 runtime missing?).")
	}
}

func openPicker(source, url string) {
	srv := &server.Server{PickURL: url, PickSource: source}
	base, err := srv.Start()
	if err != nil {
		winutil.MessageBox("Switchboard", err.Error())
		return
	}
	if !openWindow(base+"/pick", "Open with…", 480, 420, srv) {
		// No webview available: fall back to the first configured profile so
		// the click still goes somewhere.
		cfg, err := config.Load()
		if err == nil {
			for key := range cfg.Profiles {
				if route.Launch(cfg, key, url) == nil {
					route.Log(source, url, key, "no webview; arbitrary fallback")
					return
				}
			}
		}
		winutil.MessageBox("Switchboard", "No webview and no launchable profile for: "+url)
	}
}

// openWindow blocks until the window is closed. When srv is non-nil its
// OnPicked callback is wired to close the window.
func openWindow(url, title string, w, h uint, srv *server.Server) bool {
	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  title,
			Width:  w,
			Height: h,
			Center: true,
		},
	})
	if wv == nil {
		return false
	}
	if hwnd := uintptr(wv.Window()); hwnd != 0 {
		winutil.SetDarkTitleBar(hwnd, winutil.SystemPrefersDark())
	}
	if srv != nil {
		srv.OnPicked = func() {
			wv.Dispatch(func() { wv.Terminate() })
		}
	}
	wv.Navigate(url)
	wv.Run()
	return true
}

func doctor() {
	fmt.Println("Switchboard doctor")
	fmt.Println("==================")
	exe, _ := os.Executable()
	fmt.Printf("exe:        %s\n", exe)
	fmt.Printf("config:     %s\n", config.Path())
	fmt.Printf("registered: %v\n", winutil.Installed())
	fmt.Printf("default:    %v\n", winutil.IsDefaultBrowser())

	fmt.Println("\nDiscovered profiles:")
	found := profiles.Discover()
	for _, p := range found {
		fmt.Printf("  %-7s %-11s %-14s %s\n", p.Browser, p.Dir, p.Label, p.Email)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("\nCONFIG ERROR: %v\n", err)
		os.Exit(1)
	}
	problems := 0
	fmt.Println("\nConfigured profiles:")
	for key, p := range cfg.Profiles {
		ok := false
		for _, d := range found {
			if d.Browser == p.Browser && d.Dir == p.Dir {
				ok = true
				break
			}
		}
		status := "ok"
		if !ok {
			status = "MISSING (no such profile in " + p.Browser + ")"
			problems++
		}
		fmt.Printf("  %-12s -> %s %-11s %s\n", key, p.Browser, `"`+p.Dir+`"`, status)
	}
	for browser := range cfg.Browsers {
		path := cfg.Browsers[browser]
		if path == "" {
			path = profiles.BrowserPath(browser)
		}
		if path == "" {
			fmt.Printf("  browser %q: NO EXECUTABLE FOUND\n", browser)
			problems++
		}
	}
	fmt.Println("\nRules:")
	for i, r := range cfg.Rules {
		status := "ok"
		if _, ok := cfg.Profiles[r.Profile]; !ok {
			status = "BAD PROFILE"
			problems++
		}
		fmt.Printf("  %2d. source=%-14q url=%-32q -> %-12s %s\n", i+1, r.Source, r.URL, r.Profile, status)
	}
	fb := cfg.Fallback.Profile
	if _, ok := cfg.Profiles[fb]; !ok && fb != "" && fb != "ask" {
		fmt.Printf("\nfallback %q: BAD PROFILE\n", fb)
		problems++
	} else {
		fmt.Printf("\nfallback: %s\n", fb)
	}
	if problems > 0 {
		fmt.Printf("\n%d problem(s) found.\n", problems)
		os.Exit(1)
	}
	fmt.Println("\nAll good.")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
