// Package profiles discovers browser profiles by reading each Chromium
// browser's "Local State" JSON file.
package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

type Profile struct {
	Browser string `json:"browser"` // "edge" | "chrome"
	Dir     string `json:"dir"`     // profile directory name, e.g. "Profile 3"
	Label   string `json:"label"`   // display name shown in the browser
	Email   string `json:"email"`
}

func localStatePaths() map[string]string {
	l := os.Getenv("LOCALAPPDATA")
	return map[string]string{
		"edge":   filepath.Join(l, `Microsoft\Edge\User Data\Local State`),
		"chrome": filepath.Join(l, `Google\Chrome\User Data\Local State`),
	}
}

// Discover returns all profiles of all installed Chromium browsers, sorted by
// browser then directory name. Browsers that aren't installed are skipped.
func Discover() []Profile {
	var out []Profile
	for browser, path := range localStatePaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, parseLocalState(data, browser)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Browser != out[j].Browser {
			return out[i].Browser < out[j].Browser
		}
		return out[i].Dir < out[j].Dir
	})
	return out
}

func parseLocalState(data []byte, browser string) []Profile {
	var ls struct {
		Profile struct {
			InfoCache map[string]struct {
				Name     string `json:"name"`
				UserName string `json:"user_name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &ls); err != nil {
		return nil
	}
	var out []Profile
	for dir, info := range ls.Profile.InfoCache {
		out = append(out, Profile{
			Browser: browser,
			Dir:     dir,
			Label:   info.Name,
			Email:   info.UserName,
		})
	}
	return out
}

var exeNames = map[string]string{
	"edge":   "msedge.exe",
	"chrome": "chrome.exe",
}

var fallbackPaths = map[string][]string{
	"edge": {
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	},
	"chrome": {
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	},
}

// BrowserPath locates a browser executable via the App Paths registry key,
// falling back to well-known install locations. Returns "" if not found.
func BrowserPath(browser string) string {
	exe, ok := exeNames[browser]
	if !ok {
		return ""
	}
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`+exe, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		path, _, err := k.GetStringValue("")
		k.Close()
		if err == nil && fileExists(path) {
			return path
		}
	}
	for _, p := range fallbackPaths[browser] {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool {
	p = strings.Trim(p, `"`)
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
