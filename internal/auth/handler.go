package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/email"
	"github.com/igustavo11/livestreaming-clone/internal/httputil"
)

type Handler struct {
	pool          *pgxpool.Pool
	queries       *db.Queries
	cookieSecure  bool
	google        GoogleAuth
	pendingSecret string
	mailer        email.Sender
	publicBaseURL string
}

func NewHandler(pool *pgxpool.Pool, queries *db.Queries, cookieSecure bool, google GoogleAuth, pendingSecret string, mailer email.Sender, publicBaseURL string) *Handler {
	return &Handler{
		pool:          pool,
		queries:       queries,
		cookieSecure:  cookieSecure,
		google:        google,
		pendingSecret: pendingSecret,
		mailer:        mailer,
		publicBaseURL: publicBaseURL,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/signup", h.signup)
	r.Post("/login", h.login)
	r.Post("/logout", h.logout)
	r.With(h.RequireAuth).Get("/me", h.me)
	r.Get("/google", h.googleRedirect)
	r.Get("/google/callback", h.googleCallback)
	r.Post("/google/onboarding", h.googleOnboarding)
	r.Post("/forgot-password", h.forgotPassword)
	r.Post("/reset-password", h.resetPassword)
	return r
}

type userJSON struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type channelJSON struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Title    string `json:"title"`
	Category string `json:"category"`
	IsLive   bool   `json:"is_live"`
}

type authResponse struct {
	User    userJSON    `json:"user"`
	Channel channelJSON `json:"channel"`
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "invalid email")
		return
	}
	if err := ValidateUsername(req.Username); err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if len(req.Password) < MinPasswordLen {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
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
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: pgtype.Text{String: hash, Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			httputil.WriteError(w, http.StatusConflict, "email or username already taken")
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

	if err := tx.Commit(ctx); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	cookie, err := h.createSession(ctx, user.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, cookie)

	httputil.WriteJSON(w, http.StatusCreated, buildAuthResponse(httputil.UUIDString(user.ID), user.Email, user.Username, channel.ID, channel.Title, channel.Category, channel.IsLive))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	user, err := h.queries.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !user.PasswordHash.Valid || !VerifyPassword(user.PasswordHash.String, req.Password) {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Invalidate all existing sessions before creating a new one
	if err := h.queries.DeleteUserSessions(ctx, user.ID); err != nil {
		slog.ErrorContext(ctx, "failed to invalidate old sessions", "error", err)
	}

	channel, err := h.queries.GetChannelByUserID(ctx, user.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	cookie, err := h.createSession(ctx, user.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, cookie)

	httputil.WriteJSON(w, http.StatusOK, buildAuthResponse(httputil.UUIDString(user.ID), user.Email, user.Username, channel.ID, channel.Title, channel.Category, channel.IsLive))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		_ = h.queries.DeleteSession(r.Context(), HashSessionToken(c.Value))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	})

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := UserFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	channel, err := h.queries.GetChannelByUserID(r.Context(), httputil.ParseUUID(sessionUser.ID))
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildAuthResponse(sessionUser.ID, sessionUser.Email, sessionUser.Username, channel.ID, channel.Title, channel.Category, channel.IsLive))
}

// RequireAuth is middleware that loads the session user into the request context.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookieName)
		if err != nil || c.Value == "" {
			httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		su, err := ResolveSession(r.Context(), h.queries, c.Value)
		if err != nil {
			_ = h.queries.DeleteSession(r.Context(), HashSessionToken(c.Value))
			httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		next.ServeHTTP(w, r.WithContext(ContextWithUser(r.Context(), su)))
	})
}

func (h *Handler) createSession(ctx context.Context, userID pgtype.UUID) (*http.Cookie, error) {
	token, err := GenerateSessionToken()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(SessionTTL)
	err = h.queries.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: HashSessionToken(token),
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
		Expires:  expiresAt,
		MaxAge:   int(SessionTTL.Seconds()),
	}, nil
}

func buildAuthResponse(userID, email, username string, chID pgtype.UUID, title, category string, isLive bool) authResponse {
	return authResponse{
		User: userJSON{
			ID:       userID,
			Email:    email,
			Username: username,
		},
		Channel: channelJSON{
			ID:       httputil.UUIDString(chID),
			Username: username,
			Title:    title,
			Category: category,
			IsLive:   isLive,
		},
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
