package auth

import "context"

type SessionUser struct {
	ID       string
	Email    string
	Username string
}

type ctxKey struct{}

func ContextWithUser(ctx context.Context, u *SessionUser) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func UserFromContext(ctx context.Context) (*SessionUser, bool) {
	u, ok := ctx.Value(ctxKey{}).(*SessionUser)
	return u, ok
}
