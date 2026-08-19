package channel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
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

const testPendingSecret = "test-pending-secret"

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

func newRouter(store storage.ObjectStorage) http.Handler {
	if store == nil {
		store = storage.NewMemory("https://cdn.test")
	}
	return app.NewRouter(testPool, testQueries, nil, testPendingSecret, store, nil, "http://localhost", "", "")
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

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	return nil
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
	c := sessionCookie(rec)
	if c == nil {
		t.Fatal("signup missing session cookie")
	}
	return c
}

type channelResp struct {
	Channel struct {
		ID               string `json:"id"`
		Username         string `json:"username"`
		Title            string `json:"title"`
		Category         string `json:"category"`
		ThumbnailURL     string `json:"thumbnail_url"`
		IsLive           bool   `json:"is_live"`
		StreamKeyPreview string `json:"stream_key_preview"`
	} `json:"channel"`
}

func TestGetChannelRequiresAuth(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)

	rec := doJSON(t, handler, http.MethodGet, "/api/me/channel", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestGetChannelReturnsMetadata(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	rec := doJSON(t, handler, http.MethodGet, "/api/me/channel", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp channelResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Channel.ID == "" {
		t.Error("expected channel id")
	}
	if resp.Channel.Username != "streamer" {
		t.Errorf("username = %q, want streamer", resp.Channel.Username)
	}
	if resp.Channel.Title != "" {
		t.Errorf("title = %q, want empty default", resp.Channel.Title)
	}
	if resp.Channel.Category != "" {
		t.Errorf("category = %q, want empty default", resp.Channel.Category)
	}
	if resp.Channel.ThumbnailURL != "" {
		t.Errorf("thumbnail_url = %q, want empty", resp.Channel.ThumbnailURL)
	}
	if resp.Channel.IsLive {
		t.Error("expected is_live false")
	}
	if resp.Channel.StreamKeyPreview != "" {
		t.Errorf("stream_key_preview = %q, want empty until key generation", resp.Channel.StreamKeyPreview)
	}
}

func TestPutChannelUpdatesTitleAndCategory(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	rec := doJSON(t, handler, http.MethodPut, "/api/me/channel", map[string]string{
		"title":    "Friday Night Games",
		"category": "gaming",
	}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp channelResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Channel.Title != "Friday Night Games" {
		t.Errorf("title = %q", resp.Channel.Title)
	}
	if resp.Channel.Category != "gaming" {
		t.Errorf("category = %q", resp.Channel.Category)
	}

	get := doJSON(t, handler, http.MethodGet, "/api/me/channel", nil, cookie)
	if err := json.Unmarshal(get.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if resp.Channel.Title != "Friday Night Games" || resp.Channel.Category != "gaming" {
		t.Errorf("persisted metadata mismatch: %+v", resp.Channel)
	}
}

func TestPutChannelRejectsInvalidCategory(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	rec := doJSON(t, handler, http.MethodPut, "/api/me/channel", map[string]string{
		"title":    "Hello",
		"category": "not_a_real_category",
	}, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
}

func TestPutChannelAllowsEmptyCategory(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	rec := doJSON(t, handler, http.MethodPut, "/api/me/channel", map[string]string{
		"title":    "No category yet",
		"category": "",
	}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestThumbnailUploadStoresAndPersistsURL(t *testing.T) {
	cleanTables(t)
	mem := storage.NewMemory("https://cdn.test")
	handler := newRouter(mem)
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	pngBytes := tinyPNG(t)
	rec := uploadThumbnail(t, handler, cookie, "thumb.png", "image/png", pngBytes)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp channelResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.HasPrefix(resp.Channel.ThumbnailURL, "https://cdn.test/") {
		t.Errorf("thumbnail_url = %q, want cdn prefix", resp.Channel.ThumbnailURL)
	}
	if mem.Len() != 1 {
		t.Errorf("stored objects = %d, want 1", mem.Len())
	}

	get := doJSON(t, handler, http.MethodGet, "/api/me/channel", nil, cookie)
	if err := json.Unmarshal(get.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if !strings.HasPrefix(resp.Channel.ThumbnailURL, "https://cdn.test/") {
		t.Errorf("persisted thumbnail_url = %q", resp.Channel.ThumbnailURL)
	}
}

func TestThumbnailReuploadReplacesPrevious(t *testing.T) {
	cleanTables(t)
	mem := storage.NewMemory("https://cdn.test")
	handler := newRouter(mem)
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	first := uploadThumbnail(t, handler, cookie, "a.png", "image/png", tinyPNG(t))
	if first.Code != http.StatusOK {
		t.Fatalf("first upload: %d %s", first.Code, first.Body.String())
	}
	var firstResp channelResp
	_ = json.Unmarshal(first.Body.Bytes(), &firstResp)
	firstURL := firstResp.Channel.ThumbnailURL

	second := uploadThumbnail(t, handler, cookie, "b.png", "image/png", tinyPNG(t))
	if second.Code != http.StatusOK {
		t.Fatalf("second upload: %d %s", second.Code, second.Body.String())
	}
	var secondResp channelResp
	_ = json.Unmarshal(second.Body.Bytes(), &secondResp)
	if secondResp.Channel.ThumbnailURL == firstURL {
		t.Error("expected new thumbnail URL after re-upload")
	}
	if mem.Len() != 1 {
		t.Errorf("after replace, objects = %d, want 1 (old deleted)", mem.Len())
	}
}

func TestThumbnailRejectsInvalidContentType(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	rec := uploadThumbnail(t, handler, cookie, "x.gif", "image/gif", []byte("GIF89a"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
}

func TestThumbnailRejectsOversizedFile(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	big := bytes.Repeat([]byte{0xFF}, 2*1024*1024+1)
	rec := uploadThumbnail(t, handler, cookie, "big.png", "image/png", big)
	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 413 or 422; body: %s", rec.Code, rec.Body.String())
	}
}

func TestThumbnailReturns503WhenStorageUnavailable(t *testing.T) {
	cleanTables(t)
	handler := app.NewRouter(testPool, testQueries, nil, testPendingSecret, nil, nil, "http://localhost", "", "")
	cookie := signup(t, handler, "streamer@example.com", "streamer")

	rec := uploadThumbnail(t, handler, cookie, "a.png", "image/png", tinyPNG(t))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

func uploadThumbnail(t *testing.T, handler http.Handler, cookie *http.Cookie, filename, contentType string, data []byte) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="thumbnail"; filename="` + filename + `"`},
		"Content-Type":        {contentType},
	})
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/me/channel/thumbnail", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
