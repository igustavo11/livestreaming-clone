package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/app"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/dbtest"
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
	return app.NewRouter(testPool, testQueries)
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

func signupUser(t *testing.T, handler http.Handler, email, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, handler, http.MethodPost, "/api/auth/signup", map[string]string{
		"email":    email,
		"username": username,
		"password": password,
	})
}

func TestSignupCreatesUserChannelAndSession(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := signupUser(t, handler, "guguinha@example.com", "guguinha", "password123")

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	cookie := sessionCookie(rec)
	if cookie == nil {
		t.Fatal("signup did not set a session cookie")
	}
	if !cookie.HttpOnly {
		t.Error("session cookie is not HttpOnly")
	}

	var resp struct {
		User struct {
			ID       string `json:"id"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"user"`
		Channel struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			IsLive   bool   `json:"is_live"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.User.Email != "guguinha@example.com" || resp.User.Username != "guguinha" {
		t.Errorf("unexpected user in response: %+v", resp.User)
	}
	if resp.Channel.ID == "" {
		t.Error("channel was not created at signup")
	}
	if resp.Channel.Username != "guguinha" {
		t.Errorf("channel slug = %q, want guguinha", resp.Channel.Username)
	}
}

func TestSignupRejectsDuplicateUsername(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	if rec := signupUser(t, handler, "first@example.com", "guguinha", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("first signup failed: %d %s", rec.Code, rec.Body.String())
	}

	rec := signupUser(t, handler, "second@example.com", "guguinha", "password123")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestSignupRejectsDuplicateEmail(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	if rec := signupUser(t, handler, "same@example.com", "first_user", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("first signup failed: %d %s", rec.Code, rec.Body.String())
	}

	rec := signupUser(t, handler, "same@example.com", "second_user", "password123")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestSignupRejectsInvalidUsername(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := signupUser(t, handler, "user@example.com", "Bad Username!", "password123")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
}

func TestSignupRejectsShortPassword(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := signupUser(t, handler, "user@example.com", "guguinha", "short")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
}

func TestSignupResponseDoesNotLeakSecrets(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	rec := signupUser(t, handler, "user@example.com", "guguinha", "password123")
	body := rec.Body.String()

	if strings.Contains(body, "password123") || strings.Contains(body, "argon2") {
		t.Error("response leaks password or password hash")
	}
	for _, c := range rec.Result().Cookies() {
		if strings.Contains(c.String(), "password123") {
			t.Error("cookie leaks password")
		}
	}
}

func TestLoginSuccess(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	if rec := signupUser(t, handler, "user@example.com", "guguinha", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d %s", rec.Code, rec.Body.String())
	}

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": "password123",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if sessionCookie(rec) == nil {
		t.Error("login did not set a session cookie")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	if rec := signupUser(t, handler, "user@example.com", "guguinha", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d %s", rec.Code, rec.Body.String())
	}

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": "wrong-password",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if sessionCookie(rec) != nil {
		t.Error("failed login set a session cookie")
	}
}

func TestMeRequiresAuth(t *testing.T) {
	handler := newRouter()

	rec := doJSON(t, handler, http.MethodGet, "/api/auth/me", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMeReturnsUserAndChannel(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	signup := signupUser(t, handler, "user@example.com", "guguinha", "password123")
	cookie := sessionCookie(signup)
	if cookie == nil {
		t.Fatal("signup did not set a session cookie")
	}

	rec := doJSON(t, handler, http.MethodGet, "/api/auth/me", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"username":"guguinha"`) {
		t.Errorf("me response missing username: %s", body)
	}
	if !strings.Contains(body, `"channel"`) {
		t.Errorf("me response missing channel: %s", body)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	cleanTables(t)
	handler := newRouter()

	signup := signupUser(t, handler, "user@example.com", "guguinha", "password123")
	cookie := sessionCookie(signup)
	if cookie == nil {
		t.Fatal("signup did not set a session cookie")
	}

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/logout", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if rec := doJSON(t, handler, http.MethodGet, "/api/auth/me", nil, cookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("me after logout = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
