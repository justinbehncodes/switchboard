// Package route decides where a URL goes, launches the browser, and logs it.
package route

import (
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"switchboard/internal/config"
	"switchboard/internal/profiles"
	"switchboard/internal/rules"
)

// Ask is returned by Decide when no rule matched and the fallback is the
// picker.
const Ask = ""

// Decide returns the profile key for (source, url), and a human-readable
// description of what matched. Returns Ask when the picker should be shown.
func Decide(cfg *config.Config, source, url string) (profileKey, via string) {
	if idx, ok := rules.Match(cfg.Rules, source, url); ok {
		r := cfg.Rules[idx]
		if _, exists := cfg.Profiles[r.Profile]; exists {
			return r.Profile, ruleDesc(r)
		}
		return fallback(cfg, fmt.Sprintf("rule %d points at unknown profile %q", idx+1, r.Profile))
	}
	return fallback(cfg, "no rule matched")
}

func fallback(cfg *config.Config, reason string) (string, string) {
	fb := cfg.Fallback.Profile
	if fb == "" || fb == "ask" {
		return Ask, reason
	}
	if _, exists := cfg.Profiles[fb]; !exists {
		return Ask, reason + "; fallback profile missing"
	}
	return fb, reason + "; fallback"
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

// Launch opens url in the given profile's browser, detached.
func Launch(cfg *config.Config, profileKey, url string) error {
	p, ok := cfg.Profiles[profileKey]
	if !ok {
		return fmt.Errorf("unknown profile %q", profileKey)
	}
	path := cfg.Browsers[p.Browser]
	if path == "" {
		path = profiles.BrowserPath(p.Browser)
	}
	if path == "" {
		return fmt.Errorf("no executable configured for browser %q", p.Browser)
	}
	// Only http/https URLs are ever registered to us; reject anything else so
	// a crafted argument can't smuggle extra browser switches.
	if !IsWebURL(url) {
		return fmt.Errorf("refusing non-http(s) URL %q", url)
	}
	cmd := exec.Command(path, "--profile-directory="+p.Dir, url)
	return cmd.Start()
}

// IsWebURL reports whether s is an http or https URL.
func IsWebURL(s string) bool {
	l := strings.ToLower(s)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// Unwrap unrolls well-known tracking/interstitial redirectors (Google's
// /url wrapper, Outlook SafeLinks) so rules match the real destination.
func Unwrap(raw string) string {
	u, err := neturl.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.ToLower(u.Hostname())
	var target string
	switch {
	case (host == "www.google.com" || host == "google.com") && u.Path == "/url":
		target = u.Query().Get("q")
		if target == "" {
			target = u.Query().Get("url")
		}
	case strings.HasSuffix(host, ".safelinks.protection.outlook.com"):
		target = u.Query().Get("url")
	}
	if IsWebURL(target) {
		return target
	}
	return raw
}

type Entry struct {
	Time    string `json:"time"`
	Source  string `json:"source"`
	URL     string `json:"url"`
	Profile string `json:"profile"`
	Via     string `json:"via"`
}

const maxLogBytes = 256 * 1024

// Log appends one routing decision. Query strings and fragments are stripped
// before logging — they can carry tokens and add nothing to rule-writing.
func Log(source, url, profileKey, via string) {
	line := strings.Join([]string{
		time.Now().Format(time.RFC3339),
		sanitizeField(source),
		sanitizeField(SanitizeURL(url)),
		sanitizeField(profileKey),
		sanitizeField(via),
	}, "\t") + "\n"
	path := config.LogPath()
	os.MkdirAll(config.Dir(), 0o755)
	rotate(path)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line)
}

func sanitizeField(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}

// SanitizeURL strips the query string and fragment.
func SanitizeURL(url string) string {
	if i := strings.IndexAny(url, "?#"); i >= 0 {
		return url[:i]
	}
	return url
}

// rotate keeps the log bounded: when it exceeds maxLogBytes, keep the newer
// half.
func rotate(path string) {
	st, err := os.Stat(path)
	if err != nil || st.Size() <= maxLogBytes {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	half := data[len(data)/2:]
	if i := strings.IndexByte(string(half), '\n'); i >= 0 {
		half = half[i+1:]
	}
	os.WriteFile(path, half, 0o644)
}

// Tail returns the most recent n log entries, newest first.
func Tail(n int) []Entry {
	data, err := os.ReadFile(config.LogPath())
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var out []Entry
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		parts := strings.Split(lines[i], "\t")
		if len(parts) < 5 {
			continue
		}
		out = append(out, Entry{Time: parts[0], Source: parts[1], URL: parts[2], Profile: parts[3], Via: parts[4]})
	}
	return out
}
