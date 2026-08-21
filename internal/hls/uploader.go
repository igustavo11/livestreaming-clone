package hls

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/igustavo11/livestreaming-clone/internal/storage"
)

// Resolver maps a stream key to the channel's public username.
type Resolver interface {
	Resolve(streamKey string) (username string, ok bool)
}

// resolverMap is a test-friendly Resolver backed by a static map.
type resolverMap struct {
	keys map[string]string
}

func SimpleResolver(keys map[string]string) *resolverMap {
	return &resolverMap{keys: keys}
}

func (r *resolverMap) Resolve(streamKey string) (string, bool) {
	u, ok := r.keys[streamKey]
	return u, ok
}

// Uploader watches a staging directory and pushes HLS files to object storage.
type Uploader struct {
	store    storage.ObjectStorage
	resolver Resolver
	uploaded map[string]string
}

func NewUploader(store storage.ObjectStorage, resolver Resolver) *Uploader {
	return &Uploader{
		store:    store,
		resolver: resolver,
		uploaded: make(map[string]string),
	}
}

// Upload reads the file at filePath, derives the storage key from path, and pushes to storage.
// path format: "live/<streamKey>/<filename>"
func (u *Uploader) Upload(ctx context.Context, path, filePath string) error {
	streamKey, filename := parsePath(path)
	if streamKey == "" {
		return fmt.Errorf("hls: invalid path %q", path)
	}

	username, ok := u.resolver.Resolve(streamKey)
	if !ok {
		return fmt.Errorf("hls: unknown stream key %q", streamKey)
	}

	storageKey := "hls/" + username + "/" + filename

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("hls: open %s: %w", filePath, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("hls: read %s: %w", filePath, err)
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(data))
	if !strings.HasSuffix(filename, ".m3u8") && u.uploaded[storageKey] == fingerprint {
		return nil
	}

	contentType := contentTypeFor(filename)
	cacheControl := cacheControlFor(filename)

	_, err = u.store.PutCached(ctx, storageKey, contentType, cacheControl, strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return fmt.Errorf("hls: put %s: %w", storageKey, err)
	}

	// Segments are immutable — mark as uploaded to skip future re-uploads.
	// Playlists change frequently — always re-upload them.
	if !strings.HasSuffix(filename, ".m3u8") {
		u.uploaded[storageKey] = fingerprint
	}
	return nil
}

func parsePath(path string) (streamKey, filename string) {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) == 3 && parts[0] == "live" {
		return parts[1], parts[2]
	}
	parts = strings.SplitN(path, "/", 2)
	if len(parts) == 2 && strings.HasPrefix(parts[0], "live_") {
		return parts[0], parts[1]
	}
	return "", ""
}

func contentTypeFor(filename string) string {
	switch {
	case strings.HasSuffix(filename, ".m3u8"):
		return "application/vnd.apple.mpegurl"
	case strings.HasSuffix(filename, ".ts"):
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
}

func cacheControlFor(filename string) string {
	if strings.HasSuffix(filename, ".m3u8") {
		return "public, max-age=2"
	}
	return "public, max-age=31536000, immutable"
}
