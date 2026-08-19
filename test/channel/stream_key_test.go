package channel_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/igustavo11/livestreaming-clone/internal/streamkey"
)

func textHash(key string) pgtype.Text {
	return pgtype.Text{String: streamkey.Hash(key), Valid: true}
}

func TestRotateStreamKeyRequiresAuth(t *testing.T) {
	handler := newRouter(nil)
	rec := doJSON(t, handler, http.MethodPost, "/api/me/channel/stream-key", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRotateStreamKeyReturnsFullKeyOnceAndPreview(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "sk@example.com", "streamkeyer")

	rec := doJSON(t, handler, http.MethodPost, "/api/me/channel/stream-key", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		StreamKey string `json:"stream_key"`
		Channel   struct {
			StreamKeyPreview string `json:"stream_key_preview"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !strings.HasPrefix(resp.StreamKey, "live_") {
		t.Errorf("stream_key = %q, want live_ prefix", resp.StreamKey)
	}
	if resp.Channel.StreamKeyPreview == "" {
		t.Error("stream_key_preview empty after generation")
	}
	if strings.Contains(resp.Channel.StreamKeyPreview, strings.TrimPrefix(resp.StreamKey, "live_")) {
		t.Error("preview leaks full key body")
	}
	// Full key must not appear in channel object fields besides stream_key top-level.
	if strings.Contains(rec.Body.String(), `"stream_key_hash"`) {
		t.Error("response leaks stream_key_hash")
	}

	// GET only shows preview, never full key again.
	get := doJSON(t, handler, http.MethodGet, "/api/me/channel", nil, cookie)
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d", get.Code)
	}
	if strings.Contains(get.Body.String(), resp.StreamKey) {
		t.Error("GET /api/me/channel leaked full stream key")
	}
	if !strings.Contains(get.Body.String(), resp.Channel.StreamKeyPreview) {
		t.Errorf("GET missing preview %q; body: %s", resp.Channel.StreamKeyPreview, get.Body.String())
	}
}

func TestRotateStreamKeyInvalidatesPreviousKey(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "sk2@example.com", "streamkeyer2")

	first := doJSON(t, handler, http.MethodPost, "/api/me/channel/stream-key", nil, cookie)
	var firstResp struct {
		StreamKey string `json:"stream_key"`
	}
	_ = json.Unmarshal(first.Body.Bytes(), &firstResp)
	if firstResp.StreamKey == "" {
		t.Fatalf("first rotate failed: %s", first.Body.String())
	}

	// Stored hash must verify the first key.
	row, err := testQueries.GetChannelByStreamKeyHash(context.Background(), textHash(firstResp.StreamKey))
	if err != nil {
		t.Fatalf("lookup by first key hash: %v", err)
	}
	if !streamkey.Verify(firstResp.StreamKey, row.StreamKeyHash.String) {
		t.Error("first key does not verify against stored hash")
	}

	second := doJSON(t, handler, http.MethodPost, "/api/me/channel/stream-key", nil, cookie)
	var secondResp struct {
		StreamKey string `json:"stream_key"`
	}
	_ = json.Unmarshal(second.Body.Bytes(), &secondResp)
	if secondResp.StreamKey == "" || secondResp.StreamKey == firstResp.StreamKey {
		t.Fatalf("second key invalid: %q", secondResp.StreamKey)
	}

	// Old hash must no longer resolve.
	_, err = testQueries.GetChannelByStreamKeyHash(context.Background(), textHash(firstResp.StreamKey))
	if err == nil {
		t.Error("old key hash still found after rotation")
	}

	// New key verifies.
	row2, err := testQueries.GetChannelByStreamKeyHash(context.Background(), textHash(secondResp.StreamKey))
	if err != nil {
		t.Fatalf("lookup by second key hash: %v", err)
	}
	if !streamkey.Verify(secondResp.StreamKey, row2.StreamKeyHash.String) {
		t.Error("second key does not verify")
	}
}

func TestRotateStreamKeyResponseDoesNotLeakHash(t *testing.T) {
	cleanTables(t)
	handler := newRouter(nil)
	cookie := signup(t, handler, "sk3@example.com", "streamkeyer3")

	rec := doJSON(t, handler, http.MethodPost, "/api/me/channel/stream-key", nil, cookie)
	body := rec.Body.String()
	if strings.Contains(body, "argon2") || strings.Contains(body, "stream_key_hash") {
		t.Error("response leaked hash material")
	}
}
