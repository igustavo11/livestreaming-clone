package channel

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/igustavo11/livestreaming-clone/internal/auth"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/httputil"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
	"github.com/igustavo11/livestreaming-clone/internal/streamkey"
)

const (
	maxThumbnailBytes = 2 << 20 // 2 MiB
	maxTitleLen       = 140
)

var allowedThumbnailTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type Handler struct {
	queries *db.Queries
	store   storage.ObjectStorage
}

func NewHandler(queries *db.Queries, store storage.ObjectStorage) *Handler {
	return &Handler{queries: queries, store: store}
}

// Routes mounts dashboard channel endpoints. Caller must wrap with auth middleware.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.get)
	r.Put("/", h.update)
	r.Post("/thumbnail", h.uploadThumbnail)
	r.Post("/stream-key", h.rotateStreamKey)
	return r
}

type channelJSON struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	Title            string `json:"title"`
	Category         string `json:"category"`
	ThumbnailURL     string `json:"thumbnail_url"`
	IsLive           bool   `json:"is_live"`
	StreamKeyPreview string `json:"stream_key_preview"`
}

type channelResponse struct {
	Channel channelJSON `json:"channel"`
}

type updateRequest struct {
	Title    string `json:"title"`
	Category string `json:"category"`
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	row, err := h.queries.GetChannelDashboardByUserID(r.Context(), httputil.ParseUUID(user.ID))
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	preview := ""
	if row.StreamKeyPreview != "" {
		preview = row.StreamKeyPreview
	}
	httputil.WriteJSON(w, http.StatusOK, channelResponse{Channel: toJSON(row.ID, row.Username, row.Title, row.Category, row.ThumbnailUrl, row.IsLive, preview)})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req updateRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if len(req.Title) > maxTitleLen {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "title too long")
		return
	}
	req.Category = strings.TrimSpace(req.Category)
	if req.Category != "" && !ValidCategory(req.Category) {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "invalid category")
		return
	}

	ch, err := h.queries.UpdateChannelMetadata(r.Context(), db.UpdateChannelMetadataParams{
		UserID:   httputil.ParseUUID(user.ID),
		Title:    req.Title,
		Category: req.Category,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, channelResponse{Channel: toJSON(ch.ID, user.Username, ch.Title, ch.Category, ch.ThumbnailUrl, ch.IsLive, ch.StreamKeyPreview)})
}

func (h *Handler) rotateStreamKey(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	fullKey, err := streamkey.Generate()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	preview := streamkey.Preview(fullKey)
	ch, err := h.queries.UpdateChannelStreamKey(r.Context(), db.UpdateChannelStreamKeyParams{
		UserID:           httputil.ParseUUID(user.ID),
		StreamKeyHash:    pgtype.Text{String: streamkey.Hash(fullKey), Valid: true},
		StreamKeyPreview: preview,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Full key is returned exactly once at generation; never stored in plaintext.
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"stream_key": fullKey,
		"channel":    toJSON(ch.ID, user.Username, ch.Title, ch.Category, ch.ThumbnailUrl, ch.IsLive, ch.StreamKeyPreview),
	})
}

func (h *Handler) uploadThumbnail(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if h.store == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "thumbnail storage unavailable")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxThumbnailBytes+512*1024)
	if err := r.ParseMultipartForm(maxThumbnailBytes + 512*1024); err != nil {
		httputil.WriteError(w, http.StatusRequestEntityTooLarge, "thumbnail too large")
		return
	}

	file, _, err := r.FormFile("thumbnail")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "thumbnail file required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxThumbnailBytes+1))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read thumbnail")
		return
	}
	if len(data) > maxThumbnailBytes {
		httputil.WriteError(w, http.StatusRequestEntityTooLarge, "thumbnail too large")
		return
	}
	if len(data) == 0 {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "thumbnail empty")
		return
	}

	// Always validate content type against actual file contents, ignoring client header
	contentType := http.DetectContentType(data)
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	ext, ok := allowedThumbnailTypes[contentType]
	if !ok {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "thumbnail must be jpeg, png, or webp")
		return
	}

	existing, err := h.queries.GetChannelByUserID(r.Context(), httputil.ParseUUID(user.ID))
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	key := fmt.Sprintf("thumbnails/%s/%s%s", httputil.UUIDString(existing.ID), uuid.NewString(), ext)
	publicURL, err := h.store.Put(r.Context(), key, contentType, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to store thumbnail")
		return
	}

	ch, err := h.queries.UpdateChannelThumbnail(r.Context(), db.UpdateChannelThumbnailParams{
		UserID:       httputil.ParseUUID(user.ID),
		ThumbnailUrl: publicURL,
	})
	if err != nil {
		_ = h.store.Delete(r.Context(), key)
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if existing.ThumbnailUrl != "" {
		if oldKey := keyFromPublicURL(existing.ThumbnailUrl); oldKey != "" && oldKey != key {
			_ = h.store.Delete(r.Context(), oldKey)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, channelResponse{Channel: toJSON(ch.ID, user.Username, ch.Title, ch.Category, ch.ThumbnailUrl, ch.IsLive, ch.StreamKeyPreview)})
}

func toJSON(id pgtype.UUID, username, title, category, thumbnailURL string, isLive bool, streamKeyPreview string) channelJSON {
	return channelJSON{
		ID:               httputil.UUIDString(id),
		Username:         username,
		Title:            title,
		Category:         category,
		ThumbnailURL:     thumbnailURL,
		IsLive:           isLive,
		StreamKeyPreview: streamKeyPreview,
	}
}

func keyFromPublicURL(publicURL string) string {
	idx := strings.Index(publicURL, "/thumbnails/")
	if idx < 0 {
		return path.Base(publicURL)
	}
	return publicURL[idx+1:]
}
