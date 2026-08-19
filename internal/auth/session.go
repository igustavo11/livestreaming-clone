package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	SessionTTL        = 30 * 24 * time.Hour
	SessionCookieName = "session"
	MinPasswordLen    = 8
)

func GenerateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate session token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
