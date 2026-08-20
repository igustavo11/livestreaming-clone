package hls_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
