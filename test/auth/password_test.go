package auth_test

import (
	"strings"
	"testing"

	. "github.com/igustavo11/livestreaming-clone/internal/auth"
)

func TestHashPasswordProducesArgon2id(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash prefix = %q, want $argon2id$", hash[:10])
	}

	if strings.Contains(hash, "s3cret-pass") {
		t.Error("hash contains plaintext password")
	}
}

func TestVerifyPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if !VerifyPassword(hash, "correct horse") {
		t.Error("VerifyPassword() = false for correct password")
	}

	if VerifyPassword(hash, "wrong password") {
		t.Error("VerifyPassword() = true for wrong password")
	}
}

func TestVerifyPasswordRejectsGarbageHash(t *testing.T) {
	if VerifyPassword("not-a-valid-hash", "anything") {
		t.Error("VerifyPassword() = true for malformed hash")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")

	if h1 == h2 {
		t.Error("two hashes of the same password are identical (missing salt)")
	}
}
