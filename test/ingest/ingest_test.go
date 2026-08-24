package ingest_test

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

const (
	testPendingSecret  = "test-pending-secret"
	testInternalSecret = "test-internal-secret"
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
	store := storage.NewMemory("https://cdn.test")
	return app.NewRouter(app.Deps{
		Pool:           testPool,
		Queries:        testQueries,
		PendingSecret:  testPendingSecret,
		ObjectStore:    store,
		PublicBaseURL:  "http://localhost",
		InternalSecret: testInternalSecret,
		MediaMTXURL:    "http://mediamtx:9997",
	})
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, headers ...map[string]string) *httptest.ResponseRecorder {
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
	for _, h := range headers {
		for k, v := range h {
			req.Header.Set(k, v)
		}
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

func rotateKey(t *testing.T, handler http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := doJSON(t, handler, http.MethodPost, "/api/me/channel/stream-key", nil, map[string]string{"Cookie": "session=" + cookie.Value})
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		StreamKey string `json:"stream_key"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.StreamKey == "" {
		t.Fatal("rotate returned empty key")
	}
	return resp.StreamKey
}

func authHeaders(secret string) map[string]string {
	return map[string]string{"X-Internal-Secret": secret}
}

// --- MediaMTX Auth Tests ---

func TestMediamtxAuthRequiresInternalSecret(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/auth", map[string]string{
		"action": "publish",
		"path":   "live/somekey",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMediamtxAuthRejectsInvalidKey(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/auth", map[string]string{
		"action": "publish",
		"path":   "live/invalidkey123",
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestMediamtxAuthAcceptsValidKey(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	cookie := signup(t, handler, "auth@example.com", "authuser")
	key := rotateKey(t, handler, cookie)

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/auth", map[string]string{
		"action": "publish",
		"path":   "live/" + key,
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestMediamtxAuthAcceptsRootRTMPPath(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	cookie := signup(t, handler, "rootpath@example.com", "rootpath")
	key := rotateKey(t, handler, cookie)

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/auth", map[string]string{
		"action": "publish",
		"path":   key,
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusOK {
		t.Errorf("root RTMP path: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestMediamtxAuthRejectsAlreadyLive(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	cookie := signup(t, handler, "auth2@example.com", "authuser2")
	key := rotateKey(t, handler, cookie)

	// Simulate stream going live via hook
	doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "stream-available",
		"path":  "live/" + key,
	}, authHeaders(testInternalSecret))

	// Second publish attempt should be rejected
	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/auth", map[string]string{
		"action": "publish",
		"path":   "live/" + key,
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestMediamtxAuthRejectsReadAction(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	cookie := signup(t, handler, "auth3@example.com", "authuser3")
	key := rotateKey(t, handler, cookie)

	// Read action should be rejected (we only allow publish)
	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/auth", map[string]string{
		"action": "read",
		"path":   "live/" + key,
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestMediamtxAuthRejectsMissingPath(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/auth", map[string]string{
		"action": "publish",
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// --- MediaMTX Hook Tests ---

func TestHookStreamAvailableSetsLive(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	cookie := signup(t, handler, "hook@example.com", "hookuser")
	key := rotateKey(t, handler, cookie)

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "stream-available",
		"path":  "live/" + key,
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusOK {
		t.Fatalf("hook status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Verify channel is live
	get := doJSON(t, handler, http.MethodGet, "/api/me/channel", nil, map[string]string{"Cookie": "session=" + cookie.Value})
	var resp struct {
		Channel struct {
			IsLive bool `json:"is_live"`
		} `json:"channel"`
	}
	_ = json.Unmarshal(get.Body.Bytes(), &resp)
	if !resp.Channel.IsLive {
		t.Error("channel not live after stream-available hook")
	}
}

func TestHookStreamUnavailableSetsOffline(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	cookie := signup(t, handler, "hook2@example.com", "hookuser2")
	key := rotateKey(t, handler, cookie)

	// Go live
	doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "stream-available",
		"path":  "live/" + key,
	}, authHeaders(testInternalSecret))

	// Go offline
	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "stream-unavailable",
		"path":  "live/" + key,
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusOK {
		t.Fatalf("hook status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Verify channel is offline
	get := doJSON(t, handler, http.MethodGet, "/api/me/channel", nil, map[string]string{"Cookie": "session=" + cookie.Value})
	var resp struct {
		Channel struct {
			IsLive bool `json:"is_live"`
		} `json:"channel"`
	}
	_ = json.Unmarshal(get.Body.Bytes(), &resp)
	if resp.Channel.IsLive {
		t.Error("channel still live after stream-unavailable hook")
	}
}

func TestHookRequiresInternalSecret(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "stream-available",
		"path":  "live/somekey",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHookRejectsInvalidEvent(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "bogus-event",
		"path":  "live/somekey",
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHookRejectsMissingPath(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "stream-available",
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Reconciler Tests (unit-level, no MediaMTX server needed) ---

func TestReconcilerCorrectsLiveDrift(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	cookie := signup(t, handler, "recon@example.com", "reconuser")
	key := rotateKey(t, handler, cookie)

	// Manually mark channel as live without an active stream
	doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "stream-available",
		"path":  "live/" + key,
	}, authHeaders(testInternalSecret))

	// Simulate stream ending (but hook was lost)
	doJSON(t, handler, http.MethodPost, "/internal/mediamtx/hook", map[string]string{
		"event": "stream-unavailable",
		"path":  "live/" + key,
	}, authHeaders(testInternalSecret))

	// Verify channel is offline after reconciliation
	get := doJSON(t, handler, http.MethodGet, "/api/me/channel", nil, map[string]string{"Cookie": "session=" + cookie.Value})
	var resp struct {
		Channel struct {
			IsLive bool `json:"is_live"`
		} `json:"channel"`
	}
	_ = json.Unmarshal(get.Body.Bytes(), &resp)
	if resp.Channel.IsLive {
		t.Error("channel still live after stream-unavailable")
	}
}

func TestMediamtxAuthAllowsLoopbackRead(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	cookie := signup(t, handler, "read@example.com", "readuser")
	key := rotateKey(t, handler, cookie)

	// Read action from loopback IP should be allowed
	rec := doJSON(t, handler, http.MethodPost, "/internal/mediamtx/auth", map[string]string{
		"action": "read",
		"path":   "live/" + key,
		"ip":     "127.0.0.1",
	}, authHeaders(testInternalSecret))
	if rec.Code != http.StatusOK {
		t.Errorf("loopback read: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}
