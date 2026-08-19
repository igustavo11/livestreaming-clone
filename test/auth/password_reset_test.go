package auth_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/igustavo11/livestreaming-clone/internal/app"
	"github.com/igustavo11/livestreaming-clone/internal/email"
)

type fakeMailer struct {
	mu   sync.Mutex
	sent []email.Message
}

func (f *fakeMailer) Send(ctx context.Context, msg email.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, msg)
	return nil
}

func (f *fakeMailer) last() *email.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return nil
	}
	m := f.sent[len(f.sent)-1]
	return &m
}

func (f *fakeMailer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func newResetRouter(mailer email.Sender) http.Handler {
	return app.NewRouter(testPool, testQueries, nil, testPendingSecret, nil, mailer, "http://localhost")
}

func extractResetToken(body string) string {
	// Fake mailer puts the raw token in the text body after "token="
	const marker = "token="
	i := indexOf(body, marker)
	if i < 0 {
		return ""
	}
	start := i + len(marker)
	end := start
	for end < len(body) && body[end] != ' ' && body[end] != '\n' && body[end] != '"' {
		end++
	}
	return body[start:end]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestForgotPasswordAlwaysReturnsSameResponse(t *testing.T) {
	cleanTables(t)
	mailer := &fakeMailer{}
	handler := newResetRouter(mailer)

	// Unknown email
	rec1 := doJSON(t, handler, http.MethodPost, "/api/auth/forgot-password", map[string]string{
		"email": "nobody@example.com",
	})
	if rec1.Code != http.StatusOK {
		t.Fatalf("unknown email status = %d, want 200; body: %s", rec1.Code, rec1.Body.String())
	}

	// Known email
	if rec := signupUser(t, handler, "user@example.com", "resetuser", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d %s", rec.Code, rec.Body.String())
	}
	rec2 := doJSON(t, handler, http.MethodPost, "/api/auth/forgot-password", map[string]string{
		"email": "user@example.com",
	})
	if rec2.Code != http.StatusOK {
		t.Fatalf("known email status = %d, want 200; body: %s", rec2.Code, rec2.Body.String())
	}

	if rec1.Body.String() != rec2.Body.String() {
		t.Errorf("responses differ (enumeration risk):\nunknown: %s\nknown:   %s", rec1.Body.String(), rec2.Body.String())
	}
}

func TestForgotPasswordSendsEmailForExistingUser(t *testing.T) {
	cleanTables(t)
	mailer := &fakeMailer{}
	handler := newResetRouter(mailer)

	if rec := signupUser(t, handler, "user@example.com", "resetuser", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d", rec.Code)
	}

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/forgot-password", map[string]string{
		"email": "user@example.com",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if mailer.count() != 1 {
		t.Fatalf("emails sent = %d, want 1", mailer.count())
	}
	msg := mailer.last()
	if msg.To != "user@example.com" {
		t.Errorf("email To = %q, want user@example.com", msg.To)
	}
	if extractResetToken(msg.Text) == "" {
		t.Errorf("email body has no token: %s", msg.Text)
	}
}

func TestForgotPasswordDoesNotEmailUnknownUser(t *testing.T) {
	cleanTables(t)
	mailer := &fakeMailer{}
	handler := newResetRouter(mailer)

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/forgot-password", map[string]string{
		"email": "ghost@example.com",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if mailer.count() != 0 {
		t.Errorf("emails sent = %d, want 0", mailer.count())
	}
}

func TestResetPasswordWithValidToken(t *testing.T) {
	cleanTables(t)
	mailer := &fakeMailer{}
	handler := newResetRouter(mailer)

	signup := signupUser(t, handler, "user@example.com", "resetuser", "password123")
	oldCookie := sessionCookie(signup)
	if oldCookie == nil {
		t.Fatal("signup did not set session cookie")
	}

	if rec := doJSON(t, handler, http.MethodPost, "/api/auth/forgot-password", map[string]string{
		"email": "user@example.com",
	}); rec.Code != http.StatusOK {
		t.Fatalf("forgot failed: %d", rec.Code)
	}
	token := extractResetToken(mailer.last().Text)
	if token == "" {
		t.Fatal("no reset token in email")
	}

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "newpassword99",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Old session must be revoked
	if me := doJSON(t, handler, http.MethodGet, "/api/auth/me", nil, oldCookie); me.Code != http.StatusUnauthorized {
		t.Errorf("old session after reset = %d, want 401", me.Code)
	}

	// New password works
	login := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": "newpassword99",
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login with new password = %d, want 200; body: %s", login.Code, login.Body.String())
	}

	// Old password fails
	oldLogin := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": "password123",
	})
	if oldLogin.Code != http.StatusUnauthorized {
		t.Errorf("login with old password = %d, want 401", oldLogin.Code)
	}
}

func TestResetPasswordRejectsReusedToken(t *testing.T) {
	cleanTables(t)
	mailer := &fakeMailer{}
	handler := newResetRouter(mailer)

	if rec := signupUser(t, handler, "user@example.com", "resetuser", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d", rec.Code)
	}
	_ = doJSON(t, handler, http.MethodPost, "/api/auth/forgot-password", map[string]string{"email": "user@example.com"})
	token := extractResetToken(mailer.last().Text)

	if rec := doJSON(t, handler, http.MethodPost, "/api/auth/reset-password", map[string]string{
		"token": token, "password": "newpassword99",
	}); rec.Code != http.StatusOK {
		t.Fatalf("first reset failed: %d", rec.Code)
	}

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/reset-password", map[string]string{
		"token": token, "password": "anotherpass99",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("reused token status = %d, want 401", rec.Code)
	}
}

func TestResetPasswordRejectsInvalidToken(t *testing.T) {
	cleanTables(t)
	handler := newResetRouter(&fakeMailer{})

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/reset-password", map[string]string{
		"token":    "not-a-real-token",
		"password": "newpassword99",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestResetPasswordRejectsShortPassword(t *testing.T) {
	cleanTables(t)
	mailer := &fakeMailer{}
	handler := newResetRouter(mailer)

	if rec := signupUser(t, handler, "user@example.com", "resetuser", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d", rec.Code)
	}
	_ = doJSON(t, handler, http.MethodPost, "/api/auth/forgot-password", map[string]string{"email": "user@example.com"})
	token := extractResetToken(mailer.last().Text)

	rec := doJSON(t, handler, http.MethodPost, "/api/auth/reset-password", map[string]string{
		"token": token, "password": "short",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestForgotPasswordWithoutMailerStillSucceeds(t *testing.T) {
	cleanTables(t)
	handler := newResetRouter(nil)

	if rec := signupUser(t, handler, "user@example.com", "resetuser", "password123"); rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d", rec.Code)
	}

	// When mailer is nil, forgot still returns 200 (no enumeration), but cannot deliver.
	// Prefer 503 for known-user when mailer missing? Spec says always same response.
	// We'll return 200 always; delivery is best-effort / logged.
	rec := doJSON(t, handler, http.MethodPost, "/api/auth/forgot-password", map[string]string{
		"email": "user@example.com",
	})
	if rec.Code != http.StatusOK && rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 200 or 503", rec.Code)
	}
}
