package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/httputil"
)

const (
	stateCookieName = "oauth_state"
	stateTTL        = 10 * time.Minute
	googleProvider  = "google"
)

func (h *Handler) googleRedirect(w http.ResponseWriter, r *http.Request) {
	if h.google == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "google oauth not configured")
		return
	}

	state, err := randomHex(16)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
		MaxAge:   int(stateTTL.Seconds()),
	})

	http.Redirect(w, r, h.google.AuthCodeURL(state), http.StatusFound)
}

func (h *Handler) googleCallback(w http.ResponseWriter, r *http.Request) {
	if h.google == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "google oauth not configured")
		return
	}

	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		httputil.WriteError(w, http.StatusForbidden, "invalid oauth state")
		return
	}

	info, err := h.google.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil || info == nil {
		httputil.WriteError(w, http.StatusBadGateway, "google authentication failed")
		return
	}
	if !info.EmailVerified {
		httputil.WriteError(w, http.StatusForbidden, "google email is not verified")
		return
	}

	email := strings.ToLower(strings.TrimSpace(info.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "invalid email from provider")
		return
	}

	// Returning identity: log straight in.
	if identity, err := h.queries.GetIdentity(r.Context(), db.GetIdentityParams{
		Provider: googleProvider,
		Subject:  info.Subject,
	}); err == nil {
		h.finishLogin(w, r, identity.UserID)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Existing email/password account: link the identity and log in.
	if user, err := h.queries.GetUserByEmail(r.Context(), email); err == nil {
		if err := h.queries.CreateIdentity(r.Context(), db.CreateIdentityParams{
			Provider: googleProvider,
			Subject:  info.Subject,
			UserID:   user.ID,
		}); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		h.finishLogin(w, r, user.ID)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Brand-new Google user: needs a username.
	token, err := SignPending(h.pendingSecret, info.Subject, email)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     PendingCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
		MaxAge:   int(PendingTTL.Seconds()),
	})

	h.clearCookie(w, stateCookieName)
	http.Redirect(w, r, "/onboarding", http.StatusFound)
}

func (h *Handler) googleOnboarding(w http.ResponseWriter, r *http.Request) {
	pendingCookie, err := r.Cookie(PendingCookieName)
	if err != nil || pendingCookie.Value == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "pending google registration not found")
		return
	}

	pending, err := VerifyPending(h.pendingSecret, pendingCookie.Value)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "pending google registration invalid or expired")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Username string `json:"username"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := ValidateUsername(req.Username); err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.queries.WithTx(tx)

	user, err := qtx.CreateUser(ctx, db.CreateUserParams{
		Email:        pending.Email,
		Username:     req.Username,
		PasswordHash: pgtype.Text{Valid: false},
	})
	if err != nil {
		if isUniqueViolation(err) {
			httputil.WriteError(w, http.StatusConflict, "username already taken")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	channel, err := qtx.CreateChannelForUser(ctx, user.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := qtx.CreateIdentity(ctx, db.CreateIdentityParams{
		Provider: googleProvider,
		Subject:  pending.Subject,
		UserID:   user.ID,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.clearCookie(w, PendingCookieName)

	cookie, err := h.createSession(ctx, user.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, cookie)

	httputil.WriteJSON(w, http.StatusCreated, buildAuthResponse(httputil.UUIDString(user.ID), user.Email, user.Username, channel.ID, channel.Title, channel.Category, channel.AvatarUrl, channel.IsLive))
}

func (h *Handler) finishLogin(w http.ResponseWriter, r *http.Request, userID pgtype.UUID) {
	cookie, err := h.createSession(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, cookie)
	h.clearCookie(w, stateCookieName)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	})
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
