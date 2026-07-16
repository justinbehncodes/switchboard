package companion

import (
	"testing"
	"time"
)

func TestRecentlyLaunched(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	if recentlyLaunched(dir, "https://a.com/x", "home", now) {
		t.Fatal("first launch should not be a duplicate")
	}
	if !recentlyLaunched(dir, "https://a.com/x", "home", now.Add(500*time.Millisecond)) {
		t.Fatal("same url+profile inside the window should be suppressed")
	}
	if recentlyLaunched(dir, "https://a.com/x", "work", now.Add(time.Second)) {
		t.Fatal("different profile is not a duplicate")
	}
	// the work call above overwrote the record; home is fresh again
	if recentlyLaunched(dir, "https://a.com/x", "home", now.Add(2*time.Second)) {
		t.Fatal("record was replaced; home should launch again")
	}
	if recentlyLaunched(dir, "https://a.com/x", "home", now.Add(2*time.Second).Add(dedupWindow)) {
		t.Fatal("outside the window should launch")
	}
}
