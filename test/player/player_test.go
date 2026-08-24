package player_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/igustavo11/livestreaming-clone/internal/app"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
)

func newRouter() http.Handler {
	return app.NewRouter(app.Deps{
		ObjectStore:     storage.NewMemory(""),
		PublicBaseURL:   "https://cdn.test",
		R2PublicBaseURL: "https://cdn.test",
	})
}

func TestPlayerReturnsHTML(t *testing.T) {
	handler := newRouter()
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

func TestPlayerRejectsInvalidUsername(t *testing.T) {
	handler := newRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/player/../../etc/passwd", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("invalid username: status = %d, want 404", rec.Code)
	}
}
