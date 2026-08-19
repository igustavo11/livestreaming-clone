package auth

import (
	"context"
	"encoding/json"
	"errors"
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
)

type Handler struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	cookieSecure bool
}

func NewHandler(pool *pgxpool.Pool, queries *db.Queries, cookieSecure bool) *Handler {
	return &Handler{pool: pool, queries: queries, cookieSecure: cookieSecure}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/signup", h.signup)
	r.Post("/login", h.login)
	r.Post("/logout", h.logout)
	r.With(h.requireAuth).Get("/me", h.me)
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
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid email")
		return
	}
	if err := ValidateUsername(req.Username); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if len(req.Password) < MinPasswordLen {
		writeError(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.queries.WithTx(tx)

	user, err := qtx.CreateUser(ctx, db.CreateUserParams{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: hash,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "email or username already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	channel, err := qtx.CreateChannelForUser(ctx, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	cookie, err := h.createSession(ctx, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, cookie)

	writeJSON(w, http.StatusCreated, buildAuthResponse(uuidString(user.ID), user.Email, user.Username, channel.ID, channel.Title, channel.Category, channel.IsLive))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	user, err := h.queries.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !VerifyPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	channel, err := h.queries.GetChannelByUserID(ctx, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	cookie, err := h.createSession(ctx, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, cookie)

	writeJSON(w, http.StatusOK, buildAuthResponse(uuidString(user.ID), user.Email, user.Username, channel.ID, channel.Title, channel.Category, channel.IsLive))
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

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	channel, err := h.queries.GetChannelByUserID(r.Context(), parseUUID(sessionUser.ID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, buildAuthResponse(sessionUser.ID, sessionUser.Email, sessionUser.Username, channel.ID, channel.Title, channel.Category, channel.IsLive))
}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookieName)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		su, err := h.queries.GetSessionUser(r.Context(), HashSessionToken(c.Value))
		if err != nil || !su.ExpiresAt.Valid || !su.ExpiresAt.Time.After(time.Now()) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		ctx := ContextWithUser(r.Context(), &SessionUser{
			ID:       uuidString(su.ID),
			Email:    su.Email,
			Username: su.Username,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
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
			ID:       uuidString(chID),
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

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	v, err := u.Value()
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
