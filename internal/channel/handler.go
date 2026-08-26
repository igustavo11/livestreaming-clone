package channel

import (
	"bytes"
	"context"
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
	maxImageBytes = 2 << 20 // 2 MiB, shared by thumbnail and avatar uploads
	maxTitleLen   = 140
)

var allowedImageTypes = map[string]string{
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
	r.Post("/avatar", h.uploadAvatar)
	r.Post("/stream-key", h.rotateStreamKey)
	return r
}

type channelJSON struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	Title            string `json:"title"`
	Category         string `json:"category"`
	ThumbnailURL     string `json:"thumbnail_url"`
	AvatarURL        string `json:"avatar_url"`
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
	httputil.WriteJSON(w, http.StatusOK, channelResponse{Channel: toJSON(row.ID, row.Username, row.Title, row.Category, row.ThumbnailUrl, row.AvatarUrl, row.IsLive, preview)})
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

	httputil.WriteJSON(w, http.StatusOK, channelResponse{Channel: toJSON(ch.ID, user.Username, ch.Title, ch.Category, ch.ThumbnailUrl, ch.AvatarUrl, ch.IsLive, ch.StreamKeyPreview)})
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
		"channel":    toJSON(ch.ID, user.Username, ch.Title, ch.Category, ch.ThumbnailUrl, ch.AvatarUrl, ch.IsLive, ch.StreamKeyPreview),
	})
}

func (h *Handler) uploadThumbnail(w http.ResponseWriter, r *http.Request) {
	user, publicURL, key, existing, ok := h.storeUploadedImage(w, r, "thumbnail", "thumbnails")
	if !ok {
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

	h.deleteOldImage(r.Context(), existing.ThumbnailUrl, key)
	httputil.WriteJSON(w, http.StatusOK, channelResponse{Channel: toJSON(ch.ID, user.Username, ch.Title, ch.Category, ch.ThumbnailUrl, ch.AvatarUrl, ch.IsLive, ch.StreamKeyPreview)})
}

func (h *Handler) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, publicURL, key, existing, ok := h.storeUploadedImage(w, r, "avatar", "avatars")
	if !ok {
		return
	}

	ch, err := h.queries.UpdateChannelAvatar(r.Context(), db.UpdateChannelAvatarParams{
		UserID:    httputil.ParseUUID(user.ID),
		AvatarUrl: publicURL,
	})
	if err != nil {
		_ = h.store.Delete(r.Context(), key)
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.deleteOldImage(r.Context(), existing.AvatarUrl, key)
	httputil.WriteJSON(w, http.StatusOK, channelResponse{Channel: toJSON(ch.ID, user.Username, ch.Title, ch.Category, ch.ThumbnailUrl, ch.AvatarUrl, ch.IsLive, ch.StreamKeyPreview)})
}

// storeUploadedImage runs the path shared by every image upload endpoint:
// auth, validate, fetch the current channel, namespace a storage key, and
// write the file. The caller only needs to run its own field-specific
// UPDATE query afterward — the target field and returned Row type differ
// per query, which is why this stops short of doing that part too.
func (h *Handler) storeUploadedImage(w http.ResponseWriter, r *http.Request, formField, keyPrefix string) (user *auth.SessionUser, publicURL, key string, existing db.Channel, ok bool) {
	user, ok = auth.UserFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return nil, "", "", db.Channel{}, false
	}
	if h.store == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "image storage unavailable")
		return nil, "", "", db.Channel{}, false
	}

	data, contentType, ext, ok := validateImageUpload(w, r, formField)
	if !ok {
		return nil, "", "", db.Channel{}, false
	}

	existing, err := h.queries.GetChannelByUserID(r.Context(), httputil.ParseUUID(user.ID))
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return nil, "", "", db.Channel{}, false
	}

	key = fmt.Sprintf("%s/%s/%s%s", keyPrefix, httputil.UUIDString(existing.ID), uuid.NewString(), ext)
	publicURL, err = h.store.Put(r.Context(), key, contentType, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to store file")
		return nil, "", "", db.Channel{}, false
	}

	return user, publicURL, key, existing, true
}

// deleteOldImage removes the previous file once a new one has replaced it —
// a no-op if there was none, or if it somehow resolved to the same key.
func (h *Handler) deleteOldImage(ctx context.Context, oldURL, newKey string) {
	if oldURL == "" {
		return
	}
	if oldKey := keyFromPublicURL(oldURL); oldKey != "" && oldKey != newKey {
		_ = h.store.Delete(ctx, oldKey)
	}
}

// validateImageUpload reads, size-checks and content-type-sniffs an uploaded
// image file from a multipart form. Shared by thumbnail and avatar uploads,
// which differ only in what they do with the resulting bytes afterward.
func validateImageUpload(w http.ResponseWriter, r *http.Request, formField string) (data []byte, contentType, ext string, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+512*1024)
	if err := r.ParseMultipartForm(maxImageBytes + 512*1024); err != nil {
		httputil.WriteError(w, http.StatusRequestEntityTooLarge, "file too large")
		return nil, "", "", false
	}

	file, _, err := r.FormFile(formField)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, formField+" file required")
		return nil, "", "", false
	}
	defer file.Close()

	data, err = io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read file")
		return nil, "", "", false
	}
	if len(data) > maxImageBytes {
		httputil.WriteError(w, http.StatusRequestEntityTooLarge, "file too large")
		return nil, "", "", false
	}
	if len(data) == 0 {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "file empty")
		return nil, "", "", false
	}

	// Always validate content type against actual file contents, ignoring client header.
	contentType = http.DetectContentType(data)
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	ext, allowed := allowedImageTypes[contentType]
	if !allowed {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "file must be jpeg, png, or webp")
		return nil, "", "", false
	}

	return data, contentType, ext, true
}

func toJSON(id pgtype.UUID, username, title, category, thumbnailURL, avatarURL string, isLive bool, streamKeyPreview string) channelJSON {
	return channelJSON{
		ID:               httputil.UUIDString(id),
		Username:         username,
		Title:            title,
		Category:         category,
		ThumbnailURL:     thumbnailURL,
		AvatarURL:        avatarURL,
		IsLive:           isLive,
		StreamKeyPreview: streamKeyPreview,
	}
}

func keyFromPublicURL(publicURL string) string {
	idx := strings.Index(publicURL, "/thumbnails/")
	if idx < 0 {
		idx = strings.Index(publicURL, "/avatars/")
	}
	if idx < 0 {
		return path.Base(publicURL)
	}
	return publicURL[idx+1:]
}
