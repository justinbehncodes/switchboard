package companion

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"switchboard/internal/config"
	"switchboard/internal/profiles"
	"switchboard/internal/route"
)

// RunHost speaks Chrome's native messaging protocol on stdin/stdout
// (4-byte little-endian length prefix + JSON) until the browser closes the
// pipe. Config is reloaded per message so rule edits apply immediately.
func RunHost() {
	in := bufio.NewReader(os.Stdin)
	out := os.Stdout
	for {
		var length uint32
		if err := binary.Read(in, binary.LittleEndian, &length); err != nil {
			return // pipe closed
		}
		if length == 0 || length > 1<<20 {
			return
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(in, buf); err != nil {
			return
		}
		var msg Msg
		if err := json.Unmarshal(buf, &msg); err != nil {
			writeFrame(out, Resp{Action: "stay", Reason: "bad message"})
			continue
		}
		writeFrame(out, handle(msg))
	}
}

func handle(msg Msg) Resp {
	msg.URL = route.Unwrap(msg.URL)
	if !route.IsWebURL(msg.URL) {
		return Resp{Action: "stay", Reason: "not a web url"}
	}
	cfg, err := config.Load()
	if err != nil {
		return Resp{Action: "stay", Reason: "config error: " + err.Error()}
	}
	resp := Decide(cfg, profiles.Discover(), msg)
	if resp.Action != "grabbed" {
		return resp
	}
	// Chrome fires two navigation events for one click and each spawns its
	// own host process, so suppress an identical launch within the window
	// (still answer "grabbed" so the extension closes the tab).
	if recentlyLaunched(config.Dir(), msg.URL, resp.Profile, time.Now()) {
		resp.Reason += " (duplicate suppressed)"
		return resp
	}
	if err := route.Launch(cfg, resp.Profile, msg.URL); err != nil {
		return Resp{Action: "stay", Reason: "launch failed: " + err.Error()}
	}
	source := msg.From
	if source == "" {
		source = msg.Browser + "-extension"
	}
	route.Log(source, msg.URL, resp.Profile, "extension "+resp.Reason)
	return resp
}

const dedupWindow = 3 * time.Second

// recentlyLaunched reports whether the same url+profile was launched within
// dedupWindow, recording this launch otherwise. Only a hash touches disk so
// URLs (which can carry tokens) never do.
func recentlyLaunched(dir, url, profile string, now time.Time) bool {
	sum := sha256.Sum256([]byte(url + "|" + profile))
	hash := hex.EncodeToString(sum[:])
	path := filepath.Join(dir, "lastlaunch.json")
	var rec struct {
		Hash string    `json:"hash"`
		Time time.Time `json:"time"`
	}
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, &rec) == nil &&
			rec.Hash == hash && now.Sub(rec.Time) < dedupWindow {
			return true
		}
	}
	rec.Hash = hash
	rec.Time = now
	if data, err := json.Marshal(rec); err == nil {
		os.WriteFile(path, data, 0o644)
	}
	return false
}

func writeFrame(w io.Writer, resp Resp) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	binary.Write(w, binary.LittleEndian, uint32(len(data)))
	w.Write(data)
}
