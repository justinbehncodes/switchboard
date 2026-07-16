package rules

import (
	"testing"

	"switchboard/internal/config"
)

func TestGlob(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*workapp.example*", "https://app.workapp.example/login", true},
		{"*.sharepoint.com/*", "https://contoso.sharepoint.com/sites/x", true},
		{"*.sharepoint.com/*", "https://sharepoint.com/sites/x", false},
		{"*teams.microsoft.com*", "https://teams.microsoft.com/l/chat", true},
		{"https://exact.com/", "https://exact.com/", true},
		{"*github.com*", "https://gitlab.com/", false},
		{"*", "anything", true},
		{"", "anything", false},
		{"*g?thub.com*", "https://github.com/x", true},
		// regex metacharacters in the pattern must be literal
		{"*example.com*", "https://exampleXcom/", false},
	}
	for _, c := range cases {
		if got := Glob(c.pattern, c.s); got != c.want {
			t.Errorf("Glob(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

func TestMatch(t *testing.T) {
	rules := []config.Rule{
		{Source: "slack.exe", Profile: "work"},
		{Source: "ms-teams.exe", Profile: "work"},
		{URL: "*workapp.example*", Profile: "work"},
		{URL: "*home.example*", Profile: "home"},
		{Source: "olk.exe", URL: "*sharepoint*", Profile: "work"},
		{Profile: "never-matches"}, // no criteria: skipped
	}

	cases := []struct {
		name, source, url string
		wantIdx           int
		wantOK            bool
	}{
		{"source match wins first", "slack.exe", "https://home.example/x", 0, true},
		{"source is case-insensitive", "Slack.EXE", "https://x.com/", 0, true},
		{"url match", "explorer.exe", "https://app.workapp.example/", 2, true},
		{"url is case-insensitive", "explorer.exe", "HTTPS://APP.WORKAPP.EXAMPLE/", 2, true},
		{"combined source+url both required", "olk.exe", "https://foo.sharepoint.com/", 4, true},
		{"combined fails on wrong source", "notepad.exe", "https://foo.sharepoint.com/", -1, false},
		{"no match", "notepad.exe", "https://example.com/", -1, false},
		{"empty rule never matches", "", "", -1, false},
	}
	for _, c := range cases {
		idx, ok := Match(rules, c.source, c.url)
		if idx != c.wantIdx || ok != c.wantOK {
			t.Errorf("%s: Match = (%d, %v), want (%d, %v)", c.name, idx, ok, c.wantIdx, c.wantOK)
		}
	}
}
