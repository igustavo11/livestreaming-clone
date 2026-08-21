package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/config"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/dbmigrate"
	"github.com/igustavo11/livestreaming-clone/internal/hls"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
	"github.com/igustavo11/livestreaming-clone/internal/streamkey"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := dbmigrate.Up(cfg.DatabaseURL); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to create db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)

	var objectStore storage.ObjectStorage
	r2, err := storage.NewR2(storage.R2Config{
		AccountID:       cfg.R2AccountID,
		AccessKeyID:     cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket:          cfg.R2Bucket,
		PublicBaseURL:   cfg.R2PublicBaseURL,
	})
	if err != nil {
		logger.Error("failed to configure r2", "error", err)
		os.Exit(1)
	}
	if r2 != nil {
		objectStore = r2
		logger.Info("r2 enabled")
	} else {
		logger.Error("r2 not configured; hlsupload requires R2")
		os.Exit(1)
	}

	stagingDir := os.Getenv("STAGING_DIR")
	if stagingDir == "" {
		stagingDir = "/staging"
	}

	resolver := &dbResolver{queries: queries}
	up := hls.NewUploader(objectStore, resolver)

	pollInterval := 500 * time.Millisecond
	logger.Info("hlsupload starting", "staging", stagingDir, "interval", pollInterval)

	for {
		select {
		case <-ctx.Done():
			logger.Info("hlsupload stopped")
			return
		case <-time.After(pollInterval):
			if err := scanAndUpload(ctx, up, stagingDir); err != nil {
				logger.Error("scan error", "error", err)
			}
		}
	}
}

func scanAndUpload(ctx context.Context, up *hls.Uploader, stagingDir string) error {
	return filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !isHLSSegment(info.Name()) && !isHLSPlaylist(info.Name()) {
			return nil
		}

		rel, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return nil
		}

		// Convert OS path separators to slash and prefix with "live/"
		slug := strings.ReplaceAll(rel, string(filepath.Separator), "/")
		if !strings.HasPrefix(slug, "live/") && !strings.HasPrefix(slug, "live_") {
			return nil
		}

		return up.Upload(ctx, slug, path)
	})
}

func isHLSSegment(name string) bool {
	return strings.HasSuffix(name, ".ts")
}

func isHLSPlaylist(name string) bool {
	return strings.HasSuffix(name, ".m3u8")
}

// dbResolver maps stream keys to channel usernames via a single JOIN query.
type dbResolver struct {
	queries *db.Queries
}

func (r *dbResolver) Resolve(streamKey string) (string, bool) {
	username, err := r.queries.GetChannelUsernameByStreamKeyHash(context.Background(), pgtype.Text{String: streamkey.Hash(streamKey), Valid: true})
	if err != nil {
		return "", false
	}
	return username, true
}
