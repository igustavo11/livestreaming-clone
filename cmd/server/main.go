package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/app"
	"github.com/igustavo11/livestreaming-clone/internal/auth"
	"github.com/igustavo11/livestreaming-clone/internal/chat"
	"github.com/igustavo11/livestreaming-clone/internal/config"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/dbmigrate"
	"github.com/igustavo11/livestreaming-clone/internal/email"
	"github.com/igustavo11/livestreaming-clone/internal/ingest"
	"github.com/igustavo11/livestreaming-clone/internal/redact"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
)

func newRedactingHandler(base slog.Handler, secrets []string) slog.Handler {
	return redact.NewRedactingHandler(base, secrets)
}

func main() {
	// Structured JSON logs with secret redaction
	baseHandler := slog.NewJSONHandler(os.Stdout, nil)
	var secrets []string
	for _, k := range []string{"AUTH_SECRET", "INTERNAL_SECRET", "R2_SECRET_ACCESS_KEY", "RESEND_API_KEY"} {
		if v := os.Getenv(k); v != "" {
			secrets = append(secrets, v)
		}
	}
	var handler slog.Handler = baseHandler
	if len(secrets) > 0 {
		handler = newRedactingHandler(baseHandler, secrets)
	}
	logger := slog.New(handler)
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
	logger.Info("migrations applied")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to create db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping db", "error", err)
		os.Exit(1)
	}

	var google auth.GoogleAuth
	if cfg.GoogleEnabled() {
		google = auth.NewGoogleOAuth(
			cfg.GoogleClientID,
			cfg.GoogleClientSecret,
			cfg.PublicBaseURL+"/api/auth/google/callback",
		)
		logger.Info("google oauth enabled")
	}

	var objectStore storage.ObjectStorage
	r2, err := storage.NewR2(storage.R2Config{
		AccountID:       cfg.R2AccountID,
		AccessKeyID:     cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket:          cfg.R2Bucket,
		PublicBaseURL:   cfg.R2PublicBaseURL,
	})
	if err != nil {
		logger.Error("failed to configure r2 storage", "error", err)
		os.Exit(1)
	}
	if r2 != nil {
		objectStore = r2
		logger.Info("r2 object storage enabled")
	} else {
		logger.Info("r2 object storage disabled; thumbnail upload returns 503")
	}

	var mailer email.Sender
	if cfg.ResendEnabled() {
		mailer = email.NewResend(cfg.ResendAPIKey, cfg.ResendFrom)
		logger.Info("resend email enabled")
	} else {
		logger.Info("resend email disabled; forgot-password returns 503")
	}

	var chatPubSub chat.PubSub
	if cfg.RedisAddr != "" {
		ps, err := chat.NewRedisPubSub(cfg.RedisAddr)
		if err != nil {
			logger.Error("failed to connect chat redis", "addr", cfg.RedisAddr, "error", err)
			os.Exit(1)
		}
		defer ps.Close()
		chatPubSub = ps
		logger.Info("chat pub/sub via redis", "addr", cfg.RedisAddr)
	} else {
		chatPubSub = chat.NewMemoryPubSub()
		logger.Warn("REDIS_ADDR not set; chat uses in-memory pub/sub (single instance only)")
	}

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: app.NewRouter(app.Deps{
			Pool:            pool,
			Queries:         db.New(pool),
			Google:          google,
			PendingSecret:   cfg.AuthSecret,
			ObjectStore:     objectStore,
			Mailer:          mailer,
			PublicBaseURL:   cfg.PublicBaseURL,
			InternalSecret:  cfg.InternalSecret,
			MediaMTXURL:     cfg.MediaMTXURL,
			CookieSecure:    cfg.CookieSecure,
			R2PublicBaseURL: cfg.R2PublicBaseURL,
			ChatPubSub:      chatPubSub,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Start reconciler if MediaMTX is configured
	if cfg.InternalSecret != "" && cfg.MediaMTXURL != "" {
		reconciler := ingest.NewReconciler(db.New(pool), cfg.MediaMTXURL, cfg.InternalSecret, logger)
		go reconciler.Start(ctx)
	}

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
