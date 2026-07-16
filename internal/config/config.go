// Package config loads and saves the Switchboard rules file
// (%LOCALAPPDATA%\Switchboard\config.toml).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"switchboard/internal/profiles"
)

type Profile struct {
	Browser string `toml:"browser" json:"browser"`
	Dir     string `toml:"dir" json:"dir"`
	Label   string `toml:"label,omitempty" json:"label,omitempty"`
}

// Rule routes a link to a profile. Source matches the executable name of the
// app the link was clicked in (e.g. "slack.exe"); URL is a case-insensitive
// glob (* and ?) matched against the full URL. Both set = both must match.
type Rule struct {
	Source  string `toml:"source,omitempty" json:"source"`
	URL     string `toml:"url,omitempty" json:"url"`
	Profile string `toml:"profile" json:"profile"`
}

type Fallback struct {
	// Profile is a profile key, or "ask" to show the picker.
	Profile string `toml:"profile" json:"profile"`
}

type Config struct {
	Browsers map[string]string  `toml:"browsers" json:"browsers"`
	Profiles map[string]Profile `toml:"profiles" json:"profiles"`
	Rules    []Rule             `toml:"rule" json:"rules"`
	Fallback Fallback           `toml:"fallback" json:"fallback"`
}

func Dir() string     { return filepath.Join(os.Getenv("LOCALAPPDATA"), "Switchboard") }
func Path() string    { return filepath.Join(Dir(), "config.toml") }
func LogPath() string { return filepath.Join(Dir(), "route.log") }

// Load reads the config file, generating and saving a default one from the
// discovered browser profiles on first run.
func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		cfg := Default()
		if err := Save(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", Path(), err)
	}
	if cfg.Browsers == nil {
		cfg.Browsers = map[string]string{}
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Switchboard config — first matching rule wins.\n")
	b.WriteString("# source: exe the link was clicked in; url: glob against the full URL.\n\n")
	if err := toml.NewEncoder(&b).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(Path(), []byte(b.String()), 0o644)
}

// Default builds a config from whatever profiles are installed: every
// discovered profile gets a key derived from its display name, browsers get
// their detected paths, and the fallback is the first profile found.
func Default() *Config {
	cfg := &Config{
		Browsers: map[string]string{},
		Profiles: map[string]Profile{},
	}
	for _, p := range profiles.Discover() {
		if _, ok := cfg.Browsers[p.Browser]; !ok {
			cfg.Browsers[p.Browser] = profiles.BrowserPath(p.Browser)
		}
		key := Slug(p.Label)
		if key == "" {
			key = Slug(p.Browser + "-" + p.Dir)
		}
		for i := 2; ; i++ {
			if _, taken := cfg.Profiles[key]; !taken {
				break
			}
			key = fmt.Sprintf("%s-%d", key, i)
		}
		cfg.Profiles[key] = Profile{Browser: p.Browser, Dir: p.Dir, Label: p.Label}
		if cfg.Fallback.Profile == "" {
			cfg.Fallback.Profile = key
		}
	}
	return cfg
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(s string) string {
	return strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
