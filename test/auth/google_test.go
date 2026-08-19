package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/igustavo11/livestreaming-clone/internal/app"
	"github.com/igustavo11/livestreaming-clone/internal/auth"
	"github.com/igustavo11/livestreaming-clone/internal/db"
)

const testPendingSecret = "test-pending-secret"

type fakeGoogle struct {
	info *auth.GoogleUserInfo
	err  error
}

func (f *fakeGoogle) AuthCodeURL(state string) string {
	return "https://fake-google/auth?state=" + state
}

func (f *fakeGoogle) Exchange(ctx context.Context, code string) (*auth.GoogleUserInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.info, nil
}

func newOAuthRouter(g auth.GoogleAuth) http.Handler {
	return app.NewRouter(testPool, testQueries, g, testPendingSecret, nil, nil, "http://localhost", "", "", false)
}

func googleInfo(email, subject string) *auth.GoogleUserInfo {
	return &auth.GoogleUserInfo{Subject: subject, Email: email, EmailVerified: true}
}

func callbackWithState(t *testing.T, handler http.Handler, code string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google", nil)
	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, req)

	state := extractCookie(redirect, "oauth_state")
	if state == nil {
		t.Fatal("oauth_state cookie not set on redirect")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code="+code+"&state="+state.Value, nil)
	req.AddCookie(state)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func extractCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestGoogleRedirectSetsStateAndRedirects(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("x@example.com", "sub-1")})

	rec := doJSON(t, handler, http.MethodGet, "/api/auth/google", nil)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://fake-google/auth?state=") {
		t.Errorf("Location = %q, want fake google auth URL with state", loc)
	}
	if extractCookie(rec, "oauth_state") == nil {
		t.Error("oauth_state cookie not set")
	}
}

func TestGoogleNotConfiguredReturns503(t *testing.T) {
	handler := newOAuthRouter(nil)

	rec := doJSON(t, handler, http.MethodGet, "/api/auth/google", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestGoogleCallbackRejectsMismatchedState(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("x@example.com", "sub-1")})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=abc&state=wrong", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "different"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGoogleCallbackNewEmailSetsPendingCookie(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("newperson@gmail.com", "sub-new")})

	rec := callbackWithState(t, handler, "code-123")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if extractCookie(rec, "google_pending") == nil {
		t.Error("google_pending cookie not set for new user")
	}
	if extractCookie(rec, "session") != nil {
		t.Error("session cookie set before username onboarding")
	}
}

func TestGoogleCallbackRejectsUnverifiedEmail(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: &auth.GoogleUserInfo{
		Subject: "sub-1", Email: "x@example.com", EmailVerified: false,
	}})

	rec := callbackWithState(t, handler, "code-123")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGoogleCallbackExistingIdentityLogsIn(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("returning@gmail.com", "sub-return")})

	first := callbackWithState(t, handler, "code-1")
	pending := extractCookie(first, "google_pending")
	if pending == nil {
		t.Fatal("first callback did not set pending cookie")
	}
	rec := doJSON(t, handler, http.MethodPost, "/api/auth/google/onboarding",
		map[string]string{"username": "returning"}, pending)
	if rec.Code != http.StatusCreated {
		t.Fatalf("onboarding failed: %d %s", rec.Code, rec.Body.String())
	}

	second := callbackWithState(t, handler, "code-2")
	if second.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", second.Code, http.StatusFound)
	}
	if extractCookie(second, "session") == nil {
		t.Error("returning identity did not get a session cookie")
	}
	if extractCookie(second, "google_pending") != nil {
		t.Error("returning identity got a pending cookie")
	}
}

func TestGoogleCallbackLinksExistingEmailUser(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("user@example.com", "sub-google")})

	if rec := signupUser(t, handler, "user@example.com", "existing_user", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d %s", rec.Code, rec.Body.String())
	}

	rec := callbackWithState(t, handler, "code-123")
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if extractCookie(rec, "session") == nil {
		t.Error("linked account did not get a session cookie")
	}
	if extractCookie(rec, "google_pending") != nil {
		t.Error("linked account got a pending cookie")
	}

	identity, err := testQueries.GetIdentity(context.Background(), db.GetIdentityParams{
		Provider: "google",
		Subject:  "sub-google",
	})
	if err != nil {
		t.Fatalf("identity was not created: %v", err)
	}
	_ = identity
}

func TestGoogleOnboardingCreatesAccountChannelAndSession(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("fresh@gmail.com", "sub-fresh")})

	rec := callbackWithState(t, handler, "code-123")
	pending := extractCookie(rec, "google_pending")
	if pending == nil {
		t.Fatal("pending cookie not set")
	}

	rec = doJSON(t, handler, http.MethodPost, "/api/auth/google/onboarding",
		map[string]string{"username": "fresh_user"}, pending)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if extractCookie(rec, "session") == nil {
		t.Error("onboarding did not set a session cookie")
	}

	var resp struct {
		User struct {
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"user"`
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.User.Email != "fresh@gmail.com" || resp.User.Username != "fresh_user" {
		t.Errorf("unexpected user: %+v", resp.User)
	}
	if resp.Channel.ID == "" {
		t.Error("channel not created")
	}

	me := doJSON(t, handler, http.MethodGet, "/api/auth/me", nil, extractCookie(rec, "session"))
	if me.Code != http.StatusOK {
		t.Errorf("me after onboarding = %d, want 200", me.Code)
	}
}

func TestGoogleOnboardingRejectsInvalidUsername(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("fresh@gmail.com", "sub-fresh")})

	rec := callbackWithState(t, handler, "code-123")
	pending := extractCookie(rec, "google_pending")

	rec = doJSON(t, handler, http.MethodPost, "/api/auth/google/onboarding",
		map[string]string{"username": "Bad Username!"}, pending)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestGoogleOnboardingRejectsTakenUsername(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("fresh@gmail.com", "sub-fresh")})

	if rec := signupUser(t, handler, "taken@example.com", "taken_name", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d", rec.Code)
	}

	rec := callbackWithState(t, handler, "code-123")
	pending := extractCookie(rec, "google_pending")

	rec = doJSON(t, handler, http.MethodPost, "/api/auth/google/onboarding",
		map[string]string{"username": "taken_name"}, pending)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestGoogleOnboardingRequiresPendingCookie(t *testing.T) {
	cleanTables(t)
	handler := newOAuthRouter(&fakeGoogle{info: googleInfo("fresh@gmail.com", "sub-fresh")})

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/google/onboarding",
		map[string]string{"username": "fresh_user"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
