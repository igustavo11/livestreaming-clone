package observability_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/igustavo11/livestreaming-clone/internal/redact"
)

func TestRedactHandlerFiltersSecrets(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	h := redact.NewRedactingHandler(base, []string{"live_abc1234567890", "s3cr3tPass", "internal-secret-xyz"})
	logger := slog.New(h)

	logger.Info("user login", "username", "alice", "password", "s3cr3tPass")
	logger.Info("stream key", "stream_key", "live_abc1234567890", "path", "/live/live_abc1234567890")
	logger.Info("internal", "secret", "internal-secret-xyz")

	out := buf.String()
	if strings.Contains(out, "live_abc1234567890") {
		t.Fatalf("log should not contain stream key, got %s", out)
	}
	if strings.Contains(out, "s3cr3tPass") {
		t.Fatalf("log should not contain password, got %s", out)
	}
	if strings.Contains(out, "internal-secret-xyz") {
		t.Fatalf("log should not contain internal secret, got %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in output, got %s", out)
	}
	// ensure JSON and structured
	if !strings.Contains(out, `"username":"alice"`) {
		t.Fatalf("expected username preserved, got %s", out)
	}
}

func TestRedactHandlerPreservesNonSecrets(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	h := redact.NewRedactingHandler(base, []string{"secret123"})
	logger := slog.New(h)
	logger.Info("hello", "msg", "world", "count", 42)
	out := buf.String()
	if !strings.Contains(out, "world") || !strings.Contains(out, "42") {
		t.Fatalf("non-secret should be preserved, got %s", out)
	}
}
