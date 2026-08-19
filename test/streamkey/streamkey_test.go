package streamkey_test

import (
	"strings"
	"testing"

	. "github.com/igustavo11/livestreaming-clone/internal/streamkey"
)

func TestGenerateHasLivePrefixAndIsUnique(t *testing.T) {
	k1, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	k2, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !strings.HasPrefix(k1, "live_") {
		t.Errorf("key = %q, want live_ prefix", k1)
	}
	if k1 == k2 {
		t.Error("two generated keys are identical")
	}
	// base32 alphabet only after prefix
	body := strings.TrimPrefix(k1, "live_")
	if len(body) < 40 {
		t.Errorf("random body too short: %d", len(body))
	}
	for _, c := range body {
		if (c < 'a' || c > 'z') && (c < '2' || c > '7') {
			t.Errorf("unexpected char %q in key body", c)
			break
		}
	}
}

func TestHashAndVerifyRoundTrip(t *testing.T) {
	key, _ := Generate()
	hash := Hash(key)

	if !Verify(key, hash) {
		t.Error("Verify() = false for matching key")
	}
	if Verify(key+"x", hash) {
		t.Error("Verify() = true for wrong key")
	}
	if Verify(key, "") {
		t.Error("Verify() = true for empty hash")
	}
	if strings.Contains(hash, key) {
		t.Error("hash contains plaintext key")
	}
}

func TestPreviewDoesNotRevealKey(t *testing.T) {
	key := "live_abcdefghijklmnopqrstuvwxyz234567"
	preview := Preview(key)

	if preview != "live_****4567" {
		t.Errorf("Preview() = %q, want live_****4567", preview)
	}
	if strings.Contains(preview, "abcdefghijklmnopqrstuvwxyz") {
		t.Error("preview leaks random body")
	}
}
