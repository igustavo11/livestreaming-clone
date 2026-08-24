package hls_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/igustavo11/livestreaming-clone/internal/hls"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
)

func TestUploadSegmentImmutableCache(t *testing.T) {
	store := storage.NewMemory("https://cdn.test")
	resolver := hls.SimpleResolver(map[string]string{"live_abc123": "streamer1"})
	up := hls.NewUploader(store, resolver)

	seg := filepath.Join(t.TempDir(), "seg_00001.ts")
	if err := os.WriteFile(seg, []byte("fake-ts-data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := up.Upload(context.Background(), "live/live_abc123/seg_00001.ts", seg); err != nil {
		t.Fatalf("upload segment: %v", err)
	}

	key := "hls/streamer1/seg_00001.ts"
	if !store.Has(key) {
		t.Fatalf("expected key %q in store, not found", key)
	}
	data, _ := store.Get(key)
	if string(data) != "fake-ts-data" {
		t.Errorf("segment data = %q, want %q", string(data), "fake-ts-data")
	}
	cc := store.CacheControl(key)
	if cc != "public, max-age=31536000, immutable" {
		t.Errorf("segment cache-control = %q, want immutable", cc)
	}
	ct := store.ContentType(key)
	if ct != "video/mp2t" {
		t.Errorf("segment content-type = %q, want video/mp2t", ct)
	}
}

func TestUploadPlaylistShortTTL(t *testing.T) {
	store := storage.NewMemory("https://cdn.test")
	resolver := hls.SimpleResolver(map[string]string{"live_abc123": "streamer1"})
	up := hls.NewUploader(store, resolver)

	pl := filepath.Join(t.TempDir(), "index.m3u8")
	if err := os.WriteFile(pl, []byte("#EXTM3U\nseg_00001.ts"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := up.Upload(context.Background(), "live/live_abc123/index.m3u8", pl); err != nil {
		t.Fatalf("upload playlist: %v", err)
	}

	key := "hls/streamer1/index.m3u8"
	cc := store.CacheControl(key)
	if cc != "public, max-age=2" {
		t.Errorf("playlist cache-control = %q, want short TTL", cc)
	}
}

func TestPlaylistReupload(t *testing.T) {
	store := storage.NewMemory("https://cdn.test")
	resolver := hls.SimpleResolver(map[string]string{"live_abc123": "streamer1"})
	up := hls.NewUploader(store, resolver)

	pl := filepath.Join(t.TempDir(), "index.m3u8")
	if err := os.WriteFile(pl, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := up.Upload(ctx, "live/live_abc123/index.m3u8", pl); err != nil {
		t.Fatal(err)
	}
	key := "hls/streamer1/index.m3u8"
	data1, _ := store.Get(key)
	if string(data1) != "v1" {
		t.Fatalf("first upload: got %q", string(data1))
	}

	// Update content, upload again — should overwrite
	if err := os.WriteFile(pl, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := up.Upload(ctx, "live/live_abc123/index.m3u8", pl); err != nil {
		t.Fatal(err)
	}
	data2, _ := store.Get(key)
	if string(data2) != "v2" {
		t.Errorf("reupload: got %q, want v2", string(data2))
	}
}

func TestSegmentNotReuploaded(t *testing.T) {
	store := storage.NewMemory("https://cdn.test")
	resolver := hls.SimpleResolver(map[string]string{"live_abc123": "streamer1"})
	up := hls.NewUploader(store, resolver)

	seg := filepath.Join(t.TempDir(), "seg_00001.ts")
	if err := os.WriteFile(seg, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := up.Upload(ctx, "live/live_abc123/seg_00001.ts", seg); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 1 {
		t.Fatalf("after first upload: store.Len() = %d, want 1", store.Len())
	}

	// Upload same path again — should be skipped
	if err := up.Upload(ctx, "live/live_abc123/seg_00001.ts", seg); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 1 {
		t.Errorf("after reupload: store.Len() = %d, want 1 (no re-upload)", store.Len())
	}
}

func TestSegmentReuploadedWhenContentChanges(t *testing.T) {
	store := storage.NewMemory("https://cdn.test")
	resolver := hls.SimpleResolver(map[string]string{"live_abc123": "streamer1"})
	up := hls.NewUploader(store, resolver)

	seg := filepath.Join(t.TempDir(), "seg_00001.ts")
	if err := os.WriteFile(seg, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := up.Upload(ctx, "live/live_abc123/seg_00001.ts", seg); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(seg, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := up.Upload(ctx, "live/live_abc123/seg_00001.ts", seg); err != nil {
		t.Fatal(err)
	}
	data, _ := store.Get("hls/streamer1/seg_00001.ts")
	if string(data) != "v2" {
		t.Errorf("updated segment = %q, want v2", string(data))
	}
}

func TestEndStreamPurgesStorageAndFingerprints(t *testing.T) {
	store := storage.NewMemory("https://cdn.test")
	resolver := hls.SimpleResolver(map[string]string{"live_abc123": "streamer1"})
	up := hls.NewUploader(store, resolver)
	ctx := context.Background()

	dir := t.TempDir()
	seg := filepath.Join(dir, "seg_00001.ts")
	pl := filepath.Join(dir, "index.m3u8")
	if err := os.WriteFile(seg, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pl, []byte("#EXTM3U"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := up.Upload(ctx, "live/live_abc123/seg_00001.ts", seg); err != nil {
		t.Fatal(err)
	}
	if err := up.Upload(ctx, "live/live_abc123/index.m3u8", pl); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 2 {
		t.Fatalf("before EndStream: store.Len() = %d, want 2", store.Len())
	}

	if err := up.EndStream(ctx, "live_abc123"); err != nil {
		t.Fatalf("EndStream: %v", err)
	}
	if store.Len() != 0 {
		t.Errorf("after EndStream: store.Len() = %d, want 0", store.Len())
	}

	if err := up.EndStream(ctx, "live_unknown"); err == nil {
		t.Error("EndStream with unknown key: expected error")
	}
}

func writeStreamDir(t *testing.T, staging, rel string, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(staging, rel)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	pl := filepath.Join(dir, "index.m3u8")
	if err := os.WriteFile(pl, []byte("#EXTM3U"), 0644); err != nil {
		t.Fatal(err)
	}
	if age > 0 {
		past := time.Now().Add(-age)
		if err := os.Chtimes(pl, past, past); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCleanupStaleStreams(t *testing.T) {
	store := storage.NewMemory("https://cdn.test")
	resolver := hls.SimpleResolver(map[string]string{"live_abc123": "streamer1", "live_xyz789": "streamer2"})
	up := hls.NewUploader(store, resolver)
	ctx := context.Background()

	staging := t.TempDir()
	staleDir := writeStreamDir(t, staging, "live/live_abc123", time.Minute)
	freshDir := writeStreamDir(t, staging, "live/live_xyz789", 0)

	if err := up.Upload(ctx, "live/live_abc123/index.m3u8", filepath.Join(staleDir, "index.m3u8")); err != nil {
		t.Fatal(err)
	}

	ended, err := up.CleanupStaleStreams(ctx, staging, 30*time.Second)
	if err != nil {
		t.Fatalf("CleanupStaleStreams: %v", err)
	}
	if len(ended) != 1 || ended[0] != "live_abc123" {
		t.Fatalf("ended = %v, want [live_abc123]", ended)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Error("stale staging dir should be removed")
	}
	if _, err := os.Stat(filepath.Join(freshDir, "index.m3u8")); err != nil {
		t.Error("fresh staging dir should be kept")
	}
	if store.Has("hls/streamer1/index.m3u8") {
		t.Error("stale playlist should be purged from storage")
	}
}

func TestCleanupStaleStreamsRootPathLayout(t *testing.T) {
	store := storage.NewMemory("https://cdn.test")
	resolver := hls.SimpleResolver(map[string]string{"live_abc123": "streamer1"})
	up := hls.NewUploader(store, resolver)
	ctx := context.Background()

	staging := t.TempDir()
	dir := writeStreamDir(t, staging, "live_abc123", time.Minute)
	if err := up.Upload(ctx, "live_abc123/index.m3u8", filepath.Join(dir, "index.m3u8")); err != nil {
		t.Fatal(err)
	}

	ended, err := up.CleanupStaleStreams(ctx, staging, 30*time.Second)
	if err != nil {
		t.Fatalf("CleanupStaleStreams: %v", err)
	}
	if len(ended) != 1 || ended[0] != "live_abc123" {
		t.Fatalf("ended = %v, want [live_abc123]", ended)
	}
	if store.Has("hls/streamer1/index.m3u8") {
		t.Error("playlist should be purged from storage")
	}
}
