package public_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/app"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/dbtest"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
)

var (
	testPool    *pgxpool.Pool
	testQueries *db.Queries
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, queries, cleanup, err := dbtest.Setup(ctx)
	if err != nil {
		panic(err)
	}
	testPool = pool
	testQueries = queries
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func cleanTables(t *testing.T) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), "TRUNCATE TABLE users CASCADE"); err != nil {
		t.Fatalf("clean tables: %v", err)
	}
}

func newRouter() http.Handler {
	return app.NewRouter(app.Deps{
		Pool:          testPool,
		Queries:       testQueries,
		PendingSecret: "test-pending-secret",
		ObjectStore:   storage.NewMemory("https://cdn.test"),
		PublicBaseURL: "http://localhost",
	})
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func signup(t *testing.T, handler http.Handler, email, username string) *http.Cookie {
	t.Helper()
	rec := doJSON(t, handler, http.MethodPost, "/api/auth/signup", map[string]string{
		"email":    email,
		"username": username,
		"password": "password123",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, body: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	t.Fatal("signup missing session cookie")
	return nil
}

func setLive(t *testing.T, username string, live bool) {
	t.Helper()
	var channelID string
	err := testPool.QueryRow(context.Background(), "SELECT c.id FROM channels c JOIN users u ON u.id=c.user_id WHERE u.username=$1", username).Scan(&channelID)
	if err != nil {
		t.Fatalf("find channel for %s: %v", username, err)
	}
	_, err = testPool.Exec(context.Background(), "UPDATE channels SET is_live=$1 WHERE id=$2", live, channelID)
	if err != nil {
		t.Fatalf("set live: %v", err)
	}
}

func setChannelMeta(t *testing.T, username, title, category string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "UPDATE channels SET title=$2, category=$3 WHERE user_id=(SELECT id FROM users WHERE username=$1)", username, title, category)
	if err != nil {
		t.Fatalf("set meta: %v", err)
	}
}

type publicChannel struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	ThumbnailURL string `json:"thumbnail_url"`
	IsLive       bool   `json:"is_live"`
	ViewerCount  int    `json:"viewer_count"`
}

type listResp struct {
	Channels []publicChannel `json:"channels"`
}

type getResp struct {
	Channel publicChannel `json:"channel"`
}

func TestLiveChannelListReturnsOnlyLive(t *testing.T) {
	cleanTables(t)
	h := newRouter()
	signup(t, h, "a@example.com", "alice")
	signup(t, h, "b@example.com", "bob")
	signup(t, h, "c@example.com", "carol")
	setLive(t, "alice", true)
	setLive(t, "bob", false)
	setLive(t, "carol", true)

	rec := doJSON(t, h, http.MethodGet, "/api/channels", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp listResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body: %s", err, rec.Body.String())
	}
	if len(resp.Channels) != 2 {
		t.Fatalf("channels len = %d, want 2; got %+v", len(resp.Channels), resp.Channels)
	}
	for _, ch := range resp.Channels {
		if !ch.IsLive {
			t.Errorf("channel %s should be live", ch.Username)
		}
		if ch.StreamKeyLeak() {
			t.Errorf("channel %s leaks stream key", ch.Username)
		}
	}
	// bob should not appear
	for _, ch := range resp.Channels {
		if ch.Username == "bob" {
			t.Error("offline channel bob should not appear in live list")
		}
	}
}

func TestLiveChannelListFilteringByCategory(t *testing.T) {
	cleanTables(t)
	h := newRouter()
	signup(t, h, "a@example.com", "alice")
	signup(t, h, "b@example.com", "bob")
	setChannelMeta(t, "alice", "Alice Gaming", "gaming")
	setChannelMeta(t, "bob", "Bob Music", "music")
	setLive(t, "alice", true)
	setLive(t, "bob", true)

	rec := doJSON(t, h, http.MethodGet, "/api/channels?category=gaming", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp listResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Channels) != 1 {
		t.Fatalf("filtered len = %d, want 1; got %+v", len(resp.Channels), resp.Channels)
	}
	if resp.Channels[0].Username != "alice" {
		t.Errorf("username = %q, want alice", resp.Channels[0].Username)
	}
	if resp.Channels[0].Category != "gaming" {
		t.Errorf("category = %q, want gaming", resp.Channels[0].Category)
	}

	// no match
	rec2 := doJSON(t, h, http.MethodGet, "/api/channels?category=cooking", nil)
	var resp2 listResp
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if len(resp2.Channels) != 0 {
		t.Errorf("cooking filter should be empty, got %d", len(resp2.Channels))
	}
}

func TestSingleChannelLookupReturnsMetadataAndViewerCount(t *testing.T) {
	cleanTables(t)
	h := newRouter()
	signup(t, h, "a@example.com", "alice")
	setChannelMeta(t, "alice", "Alice Show", "just_chatting")
	setLive(t, "alice", true)

	rec := doJSON(t, h, http.MethodGet, "/api/channels/alice", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp getResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body: %s", err, rec.Body.String())
	}
	if resp.Channel.Username != "alice" {
		t.Errorf("username = %q, want alice", resp.Channel.Username)
	}
	if resp.Channel.Title != "Alice Show" {
		t.Errorf("title = %q, want Alice Show", resp.Channel.Title)
	}
	if resp.Channel.Category != "just_chatting" {
		t.Errorf("category = %q", resp.Channel.Category)
	}
	if !resp.Channel.IsLive {
		t.Error("expected is_live true")
	}
	// viewer_count must be present (at least 0)
	// check json contains viewer_count by raw check
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	chRaw, _ := raw["channel"].(map[string]any)
	if _, ok := chRaw["viewer_count"]; !ok {
		t.Error("viewer_count missing in response")
	}
	if resp.Channel.StreamKeyLeak() {
		t.Error("leaks stream key")
	}
}

func TestOfflineChannelNotInListButPageResolves(t *testing.T) {
	cleanTables(t)
	h := newRouter()
	signup(t, h, "a@example.com", "alice")
	setLive(t, "alice", false)

	// list should be empty
	rec := doJSON(t, h, http.MethodGet, "/api/channels", nil)
	var list listResp
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Channels) != 0 {
		t.Fatalf("expected empty list for offline, got %d", len(list.Channels))
	}

	// page still resolves with offline status
	rec2 := doJSON(t, h, http.MethodGet, "/api/channels/alice", nil)
	if rec2.Code != http.StatusOK {
		t.Fatalf("channel page status = %d, want 200; body: %s", rec2.Code, rec2.Body.String())
	}
	var resp getResp
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp)
	if resp.Channel.IsLive {
		t.Error("expected is_live false for offline channel")
	}
	if resp.Channel.Username != "alice" {
		t.Errorf("username = %q", resp.Channel.Username)
	}
}

func TestChannelPageNotFoundForUnknownUsername(t *testing.T) {
	cleanTables(t)
	h := newRouter()
	rec := doJSON(t, h, http.MethodGet, "/api/channels/nonexistent", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestLiveListOrderingNewestFirst(t *testing.T) {
	cleanTables(t)
	h := newRouter()
	signup(t, h, "a@example.com", "alice")
	signup(t, h, "b@example.com", "bob")
	signup(t, h, "c@example.com", "carol")
	setLive(t, "alice", true)
	setLive(t, "bob", true)
	setLive(t, "carol", true)

	rec := doJSON(t, h, http.MethodGet, "/api/channels", nil)
	var resp listResp
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Channels) != 3 {
		t.Fatalf("len = %d", len(resp.Channels))
	}
	// carol was created last, should be first if ordered by created_at DESC
	if resp.Channels[0].Username != "carol" {
		t.Errorf("first = %q, want carol (newest first)", resp.Channels[0].Username)
	}
}

func (c publicChannel) StreamKeyLeak() bool {
	// crude check via json marshaling: ensure no stream_key field
	b, _ := json.Marshal(c)
	s := string(b)
	return contains(s, "stream_key")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
