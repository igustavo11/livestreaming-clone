package app

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/auth"
	"github.com/igustavo11/livestreaming-clone/internal/channel"
	"github.com/igustavo11/livestreaming-clone/internal/chat"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/email"
	"github.com/igustavo11/livestreaming-clone/internal/ingest"
	"github.com/igustavo11/livestreaming-clone/internal/metrics"
	"github.com/igustavo11/livestreaming-clone/internal/public"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
)

//go:embed player.html
var playerHTML string

func NewRouter(pool *pgxpool.Pool, queries *db.Queries, google auth.GoogleAuth, pendingSecret string, store storage.ObjectStorage, mailer email.Sender, publicBaseURL string, internalSecret string, mediamtxURL string, cookieSecure bool, r2PublicBaseURL ...string) http.Handler {
	r := chi.NewRouter()
	r.Use(metrics.Middleware)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{"status": "ok"}
		code := http.StatusOK

		if _, err := queries.Ping(r.Context()); err != nil {
			status = map[string]string{"status": "degraded", "error": "database unreachable"}
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(status)
	})

	authHandler := auth.NewHandler(pool, queries, cookieSecure, google, pendingSecret, mailer, publicBaseURL)
	r.Mount("/api/auth", authHandler.Routes())

	channelHandler := channel.NewHandler(queries, store)
	r.Route("/api/me/channel", func(r chi.Router) {
		r.Use(authHandler.RequireAuth)
		r.Mount("/", channelHandler.Routes())
	})

	ingestHandler := ingest.NewHandler(queries, internalSecret, mediamtxURL)
	r.Route("/internal/mediamtx", func(r chi.Router) {
		r.Post("/auth", ingestHandler.Auth)
		r.Post("/hook", ingestHandler.Hook)
	})

	chatHandler := newChatHandler(queries)
	publicHandler := public.NewHandler(queries, chatHandler)
	r.Mount("/api/channels", publicHandler.Routes())

	r.Get("/ws/chat/{username}", chatHandler.ServeWS)

	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if cnt, err := queries.GetActiveStreamsCount(r.Context()); err == nil {
			metrics.SetActiveStreams(int(cnt))
		}
		metrics.ResetViewers()
		for ch, n := range chatHandler.Counts() {
			metrics.SetViewers(ch, n)
		}
		metrics.Handler().ServeHTTP(w, r)
	})

	var cdnBase string
	if len(r2PublicBaseURL) > 0 {
		cdnBase = strings.TrimRight(r2PublicBaseURL[0], "/")
	}
	r.Get("/player/{username}", func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		if !isValidUsername(username) {
			http.NotFound(w, r)
			return
		}
		hlsURL := cdnBase + "/hls/" + username + "/index.m3u8"
		html := strings.ReplaceAll(playerHTML, "__HLS_URL__", hlsURL)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})

	return r
}

func isValidUsername(s string) bool {
	if len(s) < 3 || len(s) > 25 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func newChatHandler(queries *db.Queries) *chat.Handler {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = os.Getenv("REDIS_URL")
	}
	if addr != "" {
		if strings.HasPrefix(addr, "redis://") {
			addr = strings.TrimPrefix(addr, "redis://")
		}
		if ps, err := chat.NewRedisPubSub(addr); err == nil {
			return chat.NewHandler(queries, ps)
		}
	}
	return chat.NewHandler(queries, chat.NewMemoryPubSub())
}
