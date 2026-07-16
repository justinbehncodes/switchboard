package profiles

import "testing"

const sampleLocalState = `{
  "profile": {
    "info_cache": {
      "Default":   {"name": "Home", "user_name": "alice@home.example"},
      "Profile 2": {"name": "Work", "user_name": "alice@work.example"},
      "Profile 3": {"name": "No Account", "user_name": ""}
    }
  },
  "unrelated": {"noise": true}
}`

func TestParseLocalState(t *testing.T) {
	got := parseLocalState([]byte(sampleLocalState), "edge")
	if len(got) != 3 {
		t.Fatalf("parsed %d profiles, want 3: %+v", len(got), got)
	}
	byDir := map[string]Profile{}
	for _, p := range got {
		if p.Browser != "edge" {
			t.Errorf("profile %q browser = %q, want edge", p.Dir, p.Browser)
		}
		byDir[p.Dir] = p
	}
	if p := byDir["Default"]; p.Label != "Home" || p.Email != "alice@home.example" {
		t.Errorf("Default parsed as %+v", p)
	}
	if p := byDir["Profile 2"]; p.Label != "Work" || p.Email != "alice@work.example" {
		t.Errorf("Profile 2 parsed as %+v", p)
	}
	if p := byDir["Profile 3"]; p.Email != "" {
		t.Errorf("Profile 3 should have empty email, got %+v", p)
	}
}

func TestParseLocalStateGarbage(t *testing.T) {
	if got := parseLocalState([]byte("not json"), "edge"); got != nil {
		t.Errorf("garbage input parsed as %+v, want nil", got)
	}
	if got := parseLocalState([]byte(`{"profile":{}}`), "edge"); len(got) != 0 {
		t.Errorf("empty info_cache parsed as %+v, want none", got)
	}
}
