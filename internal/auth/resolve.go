package auth

import (
	"context"
	"errors"
	"time"

	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/httputil"
)

// SessionStore is the minimal persistence needed to resolve a session token.
type SessionStore interface {
	GetSessionUser(ctx context.Context, tokenHash string) (db.GetSessionUserRow, error)
}

// ErrSessionInvalid means the token is unknown, malformed or expired.
var ErrSessionInvalid = errors.New("auth: invalid session")

// ResolveSession maps a raw session cookie value to its user. Both the HTTP
// middleware and the chat WebSocket upgrade resolve sessions through here.
func ResolveSession(ctx context.Context, store SessionStore, token string) (*SessionUser, error) {
	su, err := store.GetSessionUser(ctx, HashSessionToken(token))
	if err != nil {
		return nil, ErrSessionInvalid
	}
	if !su.ExpiresAt.Valid || !su.ExpiresAt.Time.After(time.Now()) {
		return nil, ErrSessionInvalid
	}
	return &SessionUser{
		ID:       httputil.UUIDString(su.ID),
		Email:    su.Email,
		Username: su.Username,
	}, nil
}
