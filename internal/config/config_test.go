package config

import (
	"os"
	"testing"
)

func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Profile 1", "profile-1"},
		{"Work (Contoso)", "work-contoso"},
		{"  spaces  ", "spaces"},
		{"ALLCAPS", "allcaps"},
		{"---", ""},
	}
	for _, c := range cases {
		if got := Slug(c.in); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())

	in := &Config{
		Browsers: map[string]string{"edge": `C:\edge.exe`, "chrome": `C:\chrome.exe`},
		Profiles: map[string]Profile{
			"home": {Browser: "chrome", Dir: "Default"},
			"work": {Browser: "edge", Dir: "Profile 1", Label: "Work"},
		},
		Rules: []Rule{
			{Source: "slack.exe", Profile: "work"},
			{URL: "*workapp.example*", Profile: "work"},
			{Source: "olk.exe", URL: "*sharepoint*", Profile: "work"},
		},
		Fallback: Fallback{Profile: "home"},
	}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(out.Rules) != len(in.Rules) {
		t.Fatalf("rules count = %d, want %d", len(out.Rules), len(in.Rules))
	}
	for i := range in.Rules {
		if out.Rules[i] != in.Rules[i] {
			t.Errorf("rule %d = %+v, want %+v (order must be preserved)", i, out.Rules[i], in.Rules[i])
		}
	}
	if out.Profiles["work"] != in.Profiles["work"] || out.Profiles["home"] != in.Profiles["home"] {
		t.Errorf("profiles = %+v, want %+v", out.Profiles, in.Profiles)
	}
	if out.Fallback != in.Fallback {
		t.Errorf("fallback = %+v, want %+v", out.Fallback, in.Fallback)
	}
	if out.Browsers["edge"] != in.Browsers["edge"] {
		t.Errorf("browsers = %+v, want %+v", out.Browsers, in.Browsers)
	}
}

func TestLoadCreatesDefaultOnFirstRun(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load on empty dir: %v", err)
	}
	if cfg.Browsers == nil || cfg.Profiles == nil {
		t.Fatal("default config has nil maps")
	}
	if _, err := os.Stat(Path()); err != nil {
		t.Fatalf("default config file not written: %v", err)
	}
	// Loading again must read the file, not regenerate.
	if _, err := Load(); err != nil {
		t.Fatalf("second Load: %v", err)
	}
}

func TestLoadRejectsInvalidTOML(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte("this is not toml ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted invalid TOML")
	}
}
