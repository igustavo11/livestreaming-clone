package chat_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/igustavo11/livestreaming-clone/internal/chat"
)

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

	// expect viewer_count broadcast to both (should be 2)
	// anon should receive viewer_count
	_ = anon.SetReadDeadline(time.Now().Add(2 * time.Second))
	var m map[string]any
	if err := anon.ReadJSON(&m); err != nil {
		t.Fatalf("anon viewer count read: %v", err)
	}
	// may receive chat history or viewer_count; skip until viewer_count
	for m["type"] != "viewer_count" {
		_ = anon.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := anon.ReadJSON(&m); err != nil {
			t.Fatalf("anon read: %v", err)
		}
	}
	if int(m["viewer_count"].(float64)) != 2 && int(m["count"].(float64)) != 2 {
		t.Fatalf("viewer_count = %v, want 2", m["viewer_count"])
	}

	// bob should also receive
	_ = bob.SetReadDeadline(time.Now().Add(2 * time.Second))
	var m2 map[string]any
	if err := bob.ReadJSON(&m2); err != nil {
		t.Fatalf("bob viewer count: %v", err)
	}
	for m2["type"] != "viewer_count" {
		_ = bob.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := bob.ReadJSON(&m2); err != nil {
			t.Fatalf("bob read: %v", err)
		}
	}
	if int(m2["viewer_count"].(float64)) != 2 {
		t.Fatalf("bob viewer_count = %v, want 2", m2["viewer_count"])
	}

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

	// disconnect one, count should decrement to 1 on next broadcast
	_ = anon.Close()
	time.Sleep(400 * time.Millisecond)
	_ = bob.SetReadDeadline(time.Now().Add(2 * time.Second))
	var m3 map[string]any
	if err := bob.ReadJSON(&m3); err != nil {
		t.Fatalf("bob after disconnect: %v", err)
	}
	for m3["type"] != "viewer_count" {
		_ = bob.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := bob.ReadJSON(&m3); err != nil {
			t.Fatalf("bob read2: %v", err)
		}
	}
	if int(m3["viewer_count"].(float64)) != 1 {
		t.Fatalf("after disconnect viewer_count = %v, want 1", m3["viewer_count"])
	}

	_ = bob.Close()
}
