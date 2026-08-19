package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	PendingCookieName = "google_pending"
	PendingTTL        = 15 * time.Minute
)

type pendingGoogle struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Exp     int64  `json:"exp"`
}

func SignPending(secret string, subject, email string) (string, error) {
	raw, err := json.Marshal(pendingGoogle{
		Subject: subject,
		Email:   email,
		Exp:     time.Now().Add(PendingTTL).Unix(),
	})
	if err != nil {
		return "", err
	}

	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + sign(secret, payload), nil
}

func VerifyPending(secret, token string) (*pendingGoogle, error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return nil, errors.New("malformed token")
	}
	if !hmac.Equal([]byte(sign(secret, payload)), []byte(sig)) {
		return nil, errors.New("invalid signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var p pendingGoogle
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	if time.Now().Unix() > p.Exp {
		return nil, errors.New("token expired")
	}

	return &p, nil
}

func sign(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
