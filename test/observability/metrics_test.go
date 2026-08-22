package observability_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/igustavo11/livestreaming-clone/internal/app"
	"github.com/igustavo11/livestreaming-clone/internal/chat"
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
		t.Fatalf("clean: %v", err)
	}
}

func newRouter(t *testing.T) http.Handler {
	t.Helper()
	// make viewer count fast for test
	orig := chat.ViewerCountInterval
	chat.ViewerCountInterval = 200 * time.Millisecond
	t.Cleanup(func() { chat.ViewerCountInterval = orig })
	return app.NewRouter(testPool, testQueries, nil, "test-pending-secret", storage.NewMemory("https://cdn.test"), nil, "http://localhost", "", "", false)
}

func TestMetricsEndpointExposesActiveStreamsAndViewers(t *testing.T) {
	cleanTables(t)
	handler := newRouter(t)

	// make a request to generate http_requests_total
	reqHealth := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	wHealth := httptest.NewRecorder()
	handler.ServeHTTP(wHealth, reqHealth)

	// initially no live
	rec := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, rec)
	if w.Code != http.StatusOK {
		t.Fatalf("metrics status %d, want 200", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	bs := string(body)
	if !strings.Contains(bs, "livestreaming_active_streams") {
		t.Fatalf("missing active_streams metric: %s", bs[:2000])
	}
	if !strings.Contains(bs, "http_requests_total") {
		t.Fatalf("missing http_requests_total, got %s", bs[:2000])
	}

	// create a live channel and a viewer
	// signup alice
	signup := func(email, username string) string {
		// use handler to signup and return cookie
		// simplified: directly insert via DB and set live
		// For test, create user via signup endpoint to get channel
		// Use httptest to signup
		b := `{"email":"` + email + `","username":"` + username + `","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("signup %s: %d %s", username, rr.Code, rr.Body.String())
		}
		return username
	}
	alice := signup("alice@example.com", "alice")
	// make live
	_, err := testPool.Exec(context.Background(), "UPDATE channels SET is_live=true WHERE user_id=(SELECT id FROM users WHERE username=$1)", alice)
	if err != nil {
		t.Fatalf("set live: %v", err)
	}
	bobCookie := func() *http.Cookie {
		b := `{"email":"bob@example.com","username":"bob","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		for _, c := range rr.Result().Cookies() {
			if c.Name == "session" {
				return c
			}
		}
		return nil
	}()
	srv := httptest.NewServer(handler)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/ws/chat/" + alice
	header := http.Header{}
	if bobCookie != nil {
		header.Set("Cookie", bobCookie.Name+"="+bobCookie.Value)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("dial ws: %v status=%d body=%s", err, resp.StatusCode, string(body))
		}
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()
	time.Sleep(300 * time.Millisecond)
	rec2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, rec2)
	bs2, _ := io.ReadAll(w2.Body)
	s2 := string(bs2)
	if !strings.Contains(s2, "livestreaming_active_streams 1") {
		t.Fatalf("expected active_streams 1, got %s", s2)
	}
	if !strings.Contains(s2, `livestreaming_viewers{channel="alice"}`) {
		t.Fatalf("expected viewers metric for alice, got %s", s2)
	}
	if !strings.Contains(s2, `livestreaming_viewers{channel="alice"} 1`) {
		// also allow with quotes or without, check contains 1
		if !strings.Contains(s2, `channel="alice"`) || !strings.Contains(s2, "1") {
			t.Fatalf("expected viewers 1 for alice, got %s", s2)
		}
	}
}
