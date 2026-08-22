package public

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/httputil"
)

type Handler struct {
	queries *db.Queries
}

func NewHandler(queries *db.Queries) *Handler {
	return &Handler{queries: queries}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{username}", h.get)
	return r
}

type channelJSON struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	ThumbnailURL string `json:"thumbnail_url"`
	IsLive       bool   `json:"is_live"`
	ViewerCount  int    `json:"viewer_count"`
}

type listResponse struct {
	Channels []channelJSON `json:"channels"`
}

type getResponse struct {
	Channel channelJSON `json:"channel"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	category := strings.TrimSpace(r.URL.Query().Get("category"))

	var channels []channelJSON
	if category != "" {
		rows, err := h.queries.ListLiveChannelsByCategory(r.Context(), category)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		channels = make([]channelJSON, 0, len(rows))
		for _, row := range rows {
			channels = append(channels, toJSON(row.ID, row.Username, row.Title, row.Category, row.ThumbnailUrl, row.IsLive))
		}
	} else {
		rows, err := h.queries.ListLiveChannels(r.Context())
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		channels = make([]channelJSON, 0, len(rows))
		for _, row := range rows {
			channels = append(channels, toJSON(row.ID, row.Username, row.Title, row.Category, row.ThumbnailUrl, row.IsLive))
		}
	}
	if channels == nil {
		channels = []channelJSON{}
	}
	httputil.WriteJSON(w, http.StatusOK, listResponse{Channels: channels})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if !isValidUsername(username) {
		httputil.WriteError(w, http.StatusNotFound, "channel not found")
		return
	}
	row, err := h.queries.GetPublicChannelByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "channel not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, getResponse{Channel: toJSON(row.ID, row.Username, row.Title, row.Category, row.ThumbnailUrl, row.IsLive)})
}

func toJSON(id pgtype.UUID, username, title, category, thumbnailURL string, isLive bool) channelJSON {
	return channelJSON{
		ID:           httputil.UUIDString(id),
		Username:     username,
		Title:        title,
		Category:     category,
		ThumbnailURL: thumbnailURL,
		IsLive:       isLive,
		ViewerCount:  0,
	}
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
