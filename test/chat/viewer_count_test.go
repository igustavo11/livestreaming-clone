package chat_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/igustavo11/livestreaming-clone/internal/chat"
)

// readViewerCount waits until conn receives a viewer_count broadcast equal to
// want. Viewer count is eventually consistent — a broadcast may reflect a
// moment before all connections registered — so we keep reading instead of
// asserting on the first broadcast.
func readViewerCount(t *testing.T, conn *websocket.Conn, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for viewer_count=%d", want)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var m map[string]any
		if err := conn.ReadJSON(&m); err != nil {
			t.Fatalf("read: %v", err)
		}
		if m["type"] != "viewer_count" {
			continue
		}
		if int(m["viewer_count"].(float64)) == want {
			return
		}
	}
}

func TestViewerCountBroadcastAndPublicAPI(t *testing.T) {
	orig := chat.ViewerCountInterval
	chat.ViewerCountInterval = 200 * time.Millisecond
	defer func() { chat.ViewerCountInterval = orig }()

	cleanTables(t)
	srv, handler := newTestServer(t)
	signup(t, handler, "alice@example.com", "alice")
	bobCookie := signup(t, handler, "bob@example.com", "bob")

	// connect 2 viewers: one anon, one auth, to alice's channel
	anon := dialWS(t, wsURL(srv, "alice"), nil)
	bob := dialWS(t, wsURL(srv, "alice"), bobCookie)
	time.Sleep(100 * time.Millisecond)

	readViewerCount(t, anon, 2)
	readViewerCount(t, bob, 2)

	// public API should also expose count
	rec := doJSON(t, handler, http.MethodGet, "/api/channels/alice", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public channel status %d", rec.Code)
	}
	var resp struct {
		Channel struct {
			ViewerCount int `json:"viewer_count"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Channel.ViewerCount != 2 {
		t.Fatalf("public viewer_count = %d, want 2", resp.Channel.ViewerCount)
	}

	// disconnect one, count should decrement to 1 on a later broadcast
	_ = anon.Close()
	readViewerCount(t, bob, 1)

	_ = bob.Close()
}
