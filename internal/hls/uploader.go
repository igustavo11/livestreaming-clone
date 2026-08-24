package hls

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	_, err = u.store.PutCached(ctx, storageKey, contentType, cacheControl, bytes.NewReader(data), int64(len(data)))
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

// ErrUnknownKey is returned when a stream key cannot be resolved to a channel.
var ErrUnknownKey = errors.New("hls: unknown stream key")

// EndStream removes all stored HLS objects for the channel publishing under
// streamKey and forgets the uploaded-segment fingerprints for that channel.
func (u *Uploader) EndStream(ctx context.Context, streamKey string) error {
	username, ok := u.resolver.Resolve(streamKey)
	if !ok {
		return fmt.Errorf("%w %q", ErrUnknownKey, streamKey)
	}

	prefix := "hls/" + username + "/"
	keys, err := u.store.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("hls: list %s: %w", prefix, err)
	}
	for _, key := range keys {
		if err := u.store.Delete(ctx, key); err != nil {
			return fmt.Errorf("hls: delete %s: %w", key, err)
		}
	}

	for key := range u.uploaded {
		if strings.HasPrefix(key, prefix) {
			delete(u.uploaded, key)
		}
	}
	return nil
}

// CleanupStaleStreams finds stream directories under stagingDir whose
// playlist has not been updated within staleAfter (i.e. the stream ended),
// purges their stored objects and removes the local files. It returns the
// stream keys that were ended.
func (u *Uploader) CleanupStaleStreams(ctx context.Context, stagingDir string, staleAfter time.Duration) ([]string, error) {
	found, err := streamDirs(stagingDir)
	if err != nil {
		return nil, err
	}

	var ended []string
	var errs []error
	for _, d := range found {
		info, err := os.Stat(filepath.Join(d, "index.m3u8"))
		if err != nil || time.Since(info.ModTime()) < staleAfter {
			continue
		}
		streamKey := streamKeyFromDir(stagingDir, d)
		if err := u.EndStream(ctx, streamKey); err != nil {
			// A rotated key cannot be purged from storage; local files still go.
			if !errors.Is(err, ErrUnknownKey) {
				errs = append(errs, fmt.Errorf("end stream %s: %w", streamKey, err))
			}
		}
		if err := os.RemoveAll(d); err != nil {
			errs = append(errs, fmt.Errorf("remove staging %s: %w", d, err))
			continue
		}
		ended = append(ended, streamKey)
	}
	return ended, errors.Join(errs...)
}

// streamDirs returns the per-stream staging directories: staging/live/<key>
// and staging/<live_key>.
func streamDirs(stagingDir string) ([]string, error) {
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir := filepath.Join(stagingDir, name)
		switch {
		case strings.HasPrefix(name, "live_"):
			dirs = append(dirs, dir)
		case name == "live":
			subs, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, s := range subs {
				if s.IsDir() {
					dirs = append(dirs, filepath.Join(dir, s.Name()))
				}
			}
		}
	}
	return dirs, nil
}

func streamKeyFromDir(stagingDir, dir string) string {
	rel, err := filepath.Rel(stagingDir, dir)
	if err != nil {
		return ""
	}
	rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")
	return strings.TrimPrefix(rel, "live/")
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
