package app

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/auth"
	"github.com/igustavo11/livestreaming-clone/internal/db"
)

func NewRouter(pool *pgxpool.Pool, queries *db.Queries) http.Handler {
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

	r.Mount("/api/auth", auth.NewHandler(pool, queries, false).Routes())

	return r
}
