package ingest

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/streamkey"
)

type Handler struct {
	queries        *db.Queries
	internalSecret string
	mediamtxURL    string
}

func NewHandler(queries *db.Queries, internalSecret string, mediamtxURL string) *Handler {
	return &Handler{
		queries:        queries,
		internalSecret: internalSecret,
		mediamtxURL:    mediamtxURL,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/auth", h.Auth)
	r.Post("/hook", h.Hook)
	return r
}

type authRequest struct {
	Action string `json:"action"`
	Path   string `json:"path"`
}

type hookRequest struct {
	Event string `json:"event"`
	Path  string `json:"path"`
}

// Auth handles MediaMTX blocking HTTP authentication.
// Returns 200 if the stream key is valid and the channel is not already live.
func (h *Handler) Auth(w http.ResponseWriter, r *http.Request) {
	if !h.checkSecret(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Only allow publish action
	if req.Action != "publish" {
		writeError(w, http.StatusForbidden, "action not allowed")
		return
	}

	key := extractKeyFromPath(req.Path)
	if key == "" {
		writeError(w, http.StatusForbidden, "invalid path")
		return
	}

	ch, err := h.queries.GetChannelByStreamKeyHash(r.Context(), pgtype.Text{String: streamkey.Hash(key), Valid: true})
	if err != nil {
		writeError(w, http.StatusForbidden, "invalid stream key")
		return
	}

	// Reject if already live
	if ch.IsLive {
		writeError(w, http.StatusConflict, "channel already live")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Hook handles MediaMTX stream-available and stream-unavailable events.
func (h *Handler) Hook(w http.ResponseWriter, r *http.Request) {
	if !h.checkSecret(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req hookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate event before doing any key lookup
	switch req.Event {
	case "stream-available", "stream-unavailable":
		// valid
	default:
		writeError(w, http.StatusBadRequest, "unknown event")
		return
	}

	key := extractKeyFromPath(req.Path)
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing or invalid path")
		return
	}

	ch, err := h.queries.GetChannelByStreamKeyHash(r.Context(), pgtype.Text{String: streamkey.Hash(key), Valid: true})
	if err != nil {
		// Key not found or invalid — ignore silently (no channel to update)
		w.WriteHeader(http.StatusOK)
		return
	}

	switch req.Event {
	case "stream-available":
		if err := h.queries.SetChannelLive(r.Context(), db.SetChannelLiveParams{
			ID:     ch.ID,
			IsLive: true,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	case "stream-unavailable":
		if err := h.queries.SetChannelLive(r.Context(), db.SetChannelLiveParams{
			ID:     ch.ID,
			IsLive: false,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) checkSecret(w http.ResponseWriter, r *http.Request) bool {
	secret := r.Header.Get("X-Internal-Secret")
	if secret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(h.internalSecret)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

func extractKeyFromPath(path string) string {
	// Path format: "live/STREAMKEY"
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] != "live" {
		return ""
	}
	return parts[1]
}

func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
