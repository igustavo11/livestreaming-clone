package auth

import (
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/email"
)

const PasswordResetTTL = time.Hour

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Always the same response — no account enumeration.
	const okMsg = "if that email is registered, a reset link has been sent"
	respondOK := func() {
		writeJSON(w, http.StatusOK, map[string]string{"status": okMsg})
	}

	// Mailer missing is a global config issue — same status for every email.
	if h.mailer == nil {
		writeError(w, http.StatusServiceUnavailable, "email delivery is not configured")
		return
	}

	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(emailAddr); err != nil {
		respondOK()
		return
	}

	user, err := h.queries.GetUserByEmail(r.Context(), emailAddr)
	if err != nil {
		respondOK()
		return
	}

	// Google-only accounts have no password; same response, no email.
	if !user.PasswordHash.Valid {
		respondOK()
		return
	}

	token, err := GenerateSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	expiresAt := time.Now().Add(PasswordResetTTL)
	if err := h.queries.CreatePasswordResetToken(r.Context(), db.CreatePasswordResetTokenParams{
		TokenHash: HashSessionToken(token),
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	base := strings.TrimRight(h.publicBaseURL, "/")
	if base == "" {
		base = "http://localhost"
	}
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", base, token)

	msg := email.Message{
		To:      emailAddr,
		Subject: "Reset your password",
		Text:    fmt.Sprintf("Use this link to reset your password (expires in 1 hour):\n%s\n\ntoken=%s\n", resetURL, token),
		HTML:    fmt.Sprintf(`<p>Use this link to reset your password (expires in 1 hour):</p><p><a href="%s">%s</a></p>`, resetURL, resetURL),
	}
	_ = h.mailer.Send(r.Context(), msg)

	respondOK()
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Password) < MinPasswordLen {
		writeError(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
		return
	}
	if strings.TrimSpace(req.Token) == "" {
		writeError(w, http.StatusUnauthorized, "invalid or expired reset token")
		return
	}

	ctx := r.Context()
	row, err := h.queries.GetPasswordResetToken(ctx, HashSessionToken(req.Token))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired reset token")
		return
	}
	if row.UsedAt.Valid {
		writeError(w, http.StatusUnauthorized, "invalid or expired reset token")
		return
	}
	if !row.ExpiresAt.Valid || !row.ExpiresAt.Time.After(time.Now()) {
		writeError(w, http.StatusUnauthorized, "invalid or expired reset token")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.queries.WithTx(tx)

	// Mark used first (single-use); if already used concurrently, stop.
	if err := qtx.MarkPasswordResetTokenUsed(ctx, row.TokenHash); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Re-check used_at after mark? Mark sets used_at only when NULL; but we don't get rows affected.
	// Re-fetch to confirm.
	check, err := qtx.GetPasswordResetToken(ctx, row.TokenHash)
	if err != nil || !check.UsedAt.Valid {
		writeError(w, http.StatusUnauthorized, "invalid or expired reset token")
		return
	}

	if err := qtx.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:           row.UserID,
		PasswordHash: pgtype.Text{String: hash, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := qtx.DeleteUserSessions(ctx, row.UserID); err != nil && err != pgx.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}
