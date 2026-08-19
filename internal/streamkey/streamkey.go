package streamkey

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"strings"
)

const prefix = "live_"

// Generate creates a new stream key: live_ + 32 random bytes as base32 (no padding).
func Generate() (fullKey string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("streamkey: generate: %w", err)
	}
	// RawStdEncoding has no padding; lowercase for OBS-friendly keys.
	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
	return prefix + encoded, nil
}

// Hash returns a hex-encoded SHA-256 of the full key (what we store).
func Hash(fullKey string) string {
	sum := sha256.Sum256([]byte(fullKey))
	return hex.EncodeToString(sum[:])
}

// Verify reports whether plain matches the stored hash (constant-time).
// Exposed for the ingest validation path (ticket #7).
func Verify(fullKey, storedHash string) bool {
	if storedHash == "" || fullKey == "" {
		return false
	}
	got := Hash(fullKey)
	return subtle.ConstantTimeCompare([]byte(got), []byte(storedHash)) == 1
}

// Preview builds a non-recoverable display form: live_**** + last 4 chars of the random part.
func Preview(fullKey string) string {
	body := strings.TrimPrefix(fullKey, prefix)
	if len(body) < 4 {
		return prefix + "****"
	}
	return prefix + "****" + body[len(body)-4:]
}
