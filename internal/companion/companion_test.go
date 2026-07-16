package companion

import (
	"testing"

	"switchboard/internal/config"
	"switchboard/internal/profiles"
)

func testConfig() *config.Config {
	return &config.Config{
		Profiles: map[string]config.Profile{
			"chrome": {Browser: "chrome", Dir: "Default"},
			"home":   {Browser: "edge", Dir: "Default"},
			"work":   {Browser: "edge", Dir: "Profile 3"},
		},
		// URL rules sit above the broad source-site rule: first match wins,
		// so a catch-all like chat.google.com must come last.
		Rules: []config.Rule{
			{Source: "slack.exe", Profile: "work"},
			{URL: "*workapp.example*", Profile: "work"},
			{URL: "*mail.google.com*", Profile: "chrome"},
			{Source: "chat.google.com", Profile: "work"},
		},
		Fallback: config.Fallback{Profile: "chrome"},
	}
}

var discovered = []profiles.Profile{
	{Browser: "chrome", Dir: "Default", Email: "alice@home.example"},
	{Browser: "edge", Dir: "Default", Email: "Alice@Home.Example"},
	{Browser: "edge", Dir: "Profile 3", Email: "alice@work.example"},
}

func TestDecide(t *testing.T) {
	cases := []struct {
		name       string
		msg        Msg
		wantAction string
		wantProf   string
	}{
		{
			"work URL clicked in chrome is grabbed",
			Msg{URL: "https://dev.workapp.example/x", From: "chat.google.com", Browser: "chrome", Email: "alice@home.example"},
			"grabbed", "work",
		},
		{
			"source-site rule grabs regardless of url",
			Msg{URL: "https://example.com/doc", From: "chat.google.com", Browser: "chrome", Email: "alice@home.example"},
			"grabbed", "work",
		},
		{
			"exe source rules never match in-browser clicks",
			Msg{URL: "https://example.com/doc", From: "reddit.com", Browser: "chrome", Email: ""},
			"stay", "",
		},
		{
			"unmatched URL stays put (fallback does not apply in-browser)",
			Msg{URL: "https://news.ycombinator.com/", From: "reddit.com", Browser: "chrome", Email: "alice@home.example"},
			"stay", "",
		},
		{
			"already in the right profile stays (case-insensitive email)",
			Msg{URL: "https://mail.google.com/mail", From: "x.com", Browser: "chrome", Email: "Alice@HOME.example"},
			"stay", "",
		},
		{
			"same browser wrong profile is grabbed",
			Msg{URL: "https://dev.workapp.example/x", From: "chat.google.com", Browser: "edge", Email: "alice@home.example"},
			"grabbed", "work",
		},
		{
			"same browser unknown profile stays",
			Msg{URL: "https://mail.google.com/mail", From: "x.com", Browser: "chrome", Email: ""},
			"stay", "",
		},
	}
	cfg := testConfig()
	for _, c := range cases {
		got := Decide(cfg, discovered, c.msg)
		if got.Action != c.wantAction || got.Profile != c.wantProf {
			t.Errorf("%s: Decide = (%s, %q), want (%s, %q) — reason: %s",
				c.name, got.Action, got.Profile, c.wantAction, c.wantProf, got.Reason)
		}
	}
}
