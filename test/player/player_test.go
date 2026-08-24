package player_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
		t.Fatalf("clean: %v", err)
	}
}

func newRouter() http.Handler {
	return app.NewRouter(app.Deps{
		Pool:            testPool,
		Queries:         testQueries,
		ObjectStore:     storage.NewMemory(""),
		PublicBaseURL:   "https://cdn.test",
		R2PublicBaseURL: "https://cdn.test",
	})
}

func signupAndSetLive(t *testing.T, h http.Handler, username string, live bool) {
	t.Helper()
	body := `{"email":"` + username + `@example.com","username":"` + username + `","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup %s: %d %s", username, rr.Code, rr.Body.String())
	}
	if live {
		if _, err := testPool.Exec(context.Background(),
			"UPDATE channels SET is_live=true WHERE user_id=(SELECT id FROM users WHERE username=$1)", username); err != nil {
			t.Fatalf("set live: %v", err)
		}
	}
}

func TestPlayerReturnsHTMLWhenLive(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	signupAndSetLive(t, handler, "streamer1", true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/player/streamer1", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "streamer1") {
		t.Error("response body does not contain channel username")
	}
	if strings.Contains(body, "__HLS_URL__") {
		t.Error("response body still contains __HLS_URL__ placeholder")
	}
	if !strings.Contains(body, "https://cdn.test/hls/streamer1/index.m3u8") {
		t.Errorf("response body does not contain expected HLS URL, got:\n%s", body)
	}
}

func TestPlayerShowsOfflinePage(t *testing.T) {
	cleanTables(t)
	handler := newRouter()
	signupAndSetLive(t, handler, "streamer1", false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/player/streamer1", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "offline") {
		t.Errorf("offline channel should render offline page, got:\n%s", body)
	}
	if strings.Contains(body, "index.m3u8") {
		t.Error("offline channel should not embed the HLS playlist")
	}
}

func TestPlayerNotFoundForUnknownChannel(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/player/ghostuser", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown channel: status = %d, want 404", rec.Code)
	}
}

func TestPlayerRejectsInvalidUsername(t *testing.T) {
	handler := newRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/player/../../etc/passwd", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("invalid username: status = %d, want 404", rec.Code)
	}
}
