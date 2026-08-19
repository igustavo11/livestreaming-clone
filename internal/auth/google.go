package auth

import "context"

type GoogleUserInfo struct {
	Subject       string
	Email         string
	EmailVerified bool
}

type GoogleAuth interface {
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (*GoogleUserInfo, error)
}
