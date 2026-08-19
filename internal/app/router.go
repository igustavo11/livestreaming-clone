package app

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/auth"
	"github.com/igustavo11/livestreaming-clone/internal/channel"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/email"
	"github.com/igustavo11/livestreaming-clone/internal/ingest"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
)

func NewRouter(pool *pgxpool.Pool, queries *db.Queries, google auth.GoogleAuth, pendingSecret string, store storage.ObjectStorage, mailer email.Sender, publicBaseURL string, internalSecret string, mediamtxURL string) http.Handler {
	r := chi.NewRouter()

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

	authHandler := auth.NewHandler(pool, queries, false, google, pendingSecret, mailer, publicBaseURL)
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

	return r
}
