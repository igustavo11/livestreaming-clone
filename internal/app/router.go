package app

import (
	_ "embed"
	"encoding/json"
	"net/http"
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

// Deps gathers everything the router needs to wire the application.
type Deps struct {
	Pool            *pgxpool.Pool
	Queries         *db.Queries
	Google          auth.GoogleAuth
	PendingSecret   string
	ObjectStore     storage.ObjectStorage
	Mailer          email.Sender
	PublicBaseURL   string
	InternalSecret  string
	MediaMTXURL     string
	CookieSecure    bool
	R2PublicBaseURL string
	ChatPubSub      chat.PubSub
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(metrics.Middleware)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{"status": "ok"}
		code := http.StatusOK

		if _, err := d.Queries.Ping(r.Context()); err != nil {
			status = map[string]string{"status": "degraded", "error": "database unreachable"}
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(status)
	})

	authHandler := auth.NewHandler(d.Pool, d.Queries, d.CookieSecure, d.Google, d.PendingSecret, d.Mailer, d.PublicBaseURL)
	r.Mount("/api/auth", authHandler.Routes())

	channelHandler := channel.NewHandler(d.Queries, d.ObjectStore)
	r.Route("/api/me/channel", func(r chi.Router) {
		r.Use(authHandler.RequireAuth)
		r.Mount("/", channelHandler.Routes())
	})

	ingestHandler := ingest.NewHandler(d.Queries, d.InternalSecret, d.MediaMTXURL)
	r.Route("/internal/mediamtx", func(r chi.Router) {
		r.Post("/auth", ingestHandler.Auth)
		r.Post("/hook", ingestHandler.Hook)
	})

	chatHandler := chat.NewHandler(d.Queries, d.ChatPubSub)
	publicHandler := public.NewHandler(d.Queries, chatHandler)
	r.Mount("/api/channels", publicHandler.Routes())

	r.Get("/ws/chat/{username}", chatHandler.ServeWS)

	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if cnt, err := d.Queries.GetActiveStreamsCount(r.Context()); err == nil {
			metrics.SetActiveStreams(int(cnt))
		}
		metrics.ResetViewers()
		for ch, n := range chatHandler.Counts() {
			metrics.SetViewers(ch, n)
		}
		metrics.Handler().ServeHTTP(w, r)
	})

	cdnBase := strings.TrimRight(d.R2PublicBaseURL, "/")
	r.Get("/player/{username}", func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		if auth.ValidateUsername(username) != nil {
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
