package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"switchboard/internal/config"
)

// setup isolates the API from the real machine: config lives in a temp
// LOCALAPPDATA for the duration of the test.
func setup(t *testing.T) *Server {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	cfg := &config.Config{
		Browsers: map[string]string{"edge": `C:\edge.exe`, "chrome": `C:\chrome.exe`},
		Profiles: map[string]config.Profile{
			"home": {Browser: "chrome", Dir: "Default"},
			"work": {Browser: "edge", Dir: "Profile 1"},
		},
		Rules: []config.Rule{
			{Source: "slack.exe", Profile: "work"},
			{URL: "*workapp.example*", Profile: "work"},
		},
		Fallback: config.Fallback{Profile: "home"},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return &Server{}
}

func postJSON(t *testing.T, handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func TestHandleTest(t *testing.T) {
	s := setup(t)
	cases := []struct {
		body        string
		wantProfile string
	}{
		{`{"url":"https://x.workapp.example/a","source":""}`, "work"},
		{`{"url":"https://anything.example/","source":"slack.exe"}`, "work"},
		{`{"url":"https://anything.example/","source":"notepad.exe"}`, "home"},
	}
	for _, c := range cases {
		w := postJSON(t, s.handleTest, "/api/test", c.body)
		if w.Code != 200 {
			t.Fatalf("%s: status %d: %s", c.body, w.Code, w.Body.String())
		}
		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["profile"] != c.wantProfile {
			t.Errorf("%s -> profile %q, want %q (via %q)", c.body, resp["profile"], c.wantProfile, resp["via"])
		}
	}
}

func TestHandleSaveConfigValidates(t *testing.T) {
	s := setup(t)

	w := postJSON(t, s.handleSaveConfig, "/api/config",
		`{"browsers":{},"profiles":{"home":{"browser":"chrome","dir":"Default"}},"rules":[{"source":"a.exe","url":"","profile":"ghost"}],"fallback":{"profile":"home"}}`)
	if w.Code != 400 {
		t.Fatalf("rule with unknown profile accepted: status %d", w.Code)
	}

	w = postJSON(t, s.handleSaveConfig, "/api/config",
		`{"browsers":{},"profiles":{"home":{"browser":"chrome","dir":"Default"}},"rules":[{"source":"a.exe","url":"","profile":"home"}],"fallback":{"profile":"home"}}`)
	if w.Code != 200 {
		t.Fatalf("valid config rejected: status %d: %s", w.Code, w.Body.String())
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Source != "a.exe" {
		t.Errorf("saved config not persisted correctly: %+v", cfg.Rules)
	}
}

func TestHandleSaveConfigRejectsBadJSON(t *testing.T) {
	s := setup(t)
	if w := postJSON(t, s.handleSaveConfig, "/api/config", "{not json"); w.Code != 400 {
		t.Fatalf("bad JSON accepted: status %d", w.Code)
	}
}

func TestHandleState(t *testing.T) {
	s := setup(t)
	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()
	s.handleState(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp stateResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Config == nil || len(resp.Config.Profiles) != 2 {
		t.Errorf("state config = %+v, want 2 profiles", resp.Config)
	}
	if resp.ConfigPath == "" {
		t.Error("state missing config path")
	}
}
