package public

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/igustavo11/livestreaming-clone/internal/auth"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/httputil"
)

const (
	defaultListLimit = 24
	maxListLimit     = 50
	// Bounds offset well under int32 so it can never wrap negative when cast
	// for the query param, which would surface as a Postgres error.
	maxListOffset = 1_000_000
)

type ViewerCounter interface {
	Count(channel string) int
}

type Handler struct {
	queries *db.Queries
	counter ViewerCounter
}

func NewHandler(queries *db.Queries, counter ViewerCounter) *Handler {
	return &Handler{queries: queries, counter: counter}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{username}", h.get)
	r.Post("/{username}/follow", h.follow)
	r.Delete("/{username}/follow", h.unfollow)
	return r
}

type channelJSON struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Title         string `json:"title"`
	Category      string `json:"category"`
	ThumbnailURL  string `json:"thumbnail_url"`
	AvatarURL     string `json:"avatar_url"`
	IsLive        bool   `json:"is_live"`
	ViewerCount   int    `json:"viewer_count"`
	FollowerCount int64  `json:"follower_count"`
	IsFollowing   bool   `json:"is_following"`
}

type listResponse struct {
	Channels []channelJSON `json:"channels"`
	HasMore  bool          `json:"has_more"`
}

type getResponse struct {
	Channel channelJSON `json:"channel"`
}

type followResponse struct {
	FollowerCount int64 `json:"follower_count"`
	IsFollowing   bool  `json:"is_following"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	limit := parseIntParam(r.URL.Query().Get("limit"), defaultListLimit, maxListLimit)
	offset := parseIntParam(r.URL.Query().Get("offset"), 0, maxListOffset)

	params := db.ListLiveChannelsFilteredParams{
		// Fetch one extra row to cheaply derive has_more without a COUNT query.
		Limit:  int32(limit + 1),
		Offset: int32(offset),
	}
	if category != "" {
		params.Category = pgtype.Text{String: category, Valid: true}
	}
	if search != "" {
		params.Search = pgtype.Text{String: search, Valid: true}
	}

	rows, err := h.queries.ListLiveChannelsFiltered(r.Context(), params)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	viewer := h.resolveOptionalUser(r)

	channels := make([]channelJSON, 0, len(rows))
	for _, row := range rows {
		channels = append(channels, h.toJSON(
			row.ID, row.Username, row.Title, row.Category, row.ThumbnailUrl, row.AvatarUrl,
			row.IsLive, row.FollowerCount, h.isFollowing(r, viewer, row.ID),
		))
	}
	httputil.WriteJSON(w, http.StatusOK, listResponse{Channels: channels, HasMore: hasMore})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if auth.ValidateUsername(username) != nil {
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
	viewer := h.resolveOptionalUser(r)
	httputil.WriteJSON(w, http.StatusOK, getResponse{Channel: h.toJSON(
		row.ID, row.Username, row.Title, row.Category, row.ThumbnailUrl, row.AvatarUrl,
		row.IsLive, row.FollowerCount, h.isFollowing(r, viewer, row.ID),
	)})
}

func (h *Handler) follow(w http.ResponseWriter, r *http.Request) {
	h.setFollow(w, r, true)
}

func (h *Handler) unfollow(w http.ResponseWriter, r *http.Request) {
	h.setFollow(w, r, false)
}

func (h *Handler) setFollow(w http.ResponseWriter, r *http.Request, follow bool) {
	viewer := h.resolveOptionalUser(r)
	if viewer == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	username := chi.URLParam(r, "username")
	if auth.ValidateUsername(username) != nil {
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

	if httputil.UUIDString(row.UserID) == viewer.ID {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "cannot follow your own channel")
		return
	}

	followerID := httputil.ParseUUID(viewer.ID)
	if follow {
		err = h.queries.FollowChannel(r.Context(), db.FollowChannelParams{FollowerUserID: followerID, ChannelID: row.ID})
	} else {
		err = h.queries.UnfollowChannel(r.Context(), db.UnfollowChannelParams{FollowerUserID: followerID, ChannelID: row.ID})
	}
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	count, err := h.queries.GetFollowerCount(r.Context(), row.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, followResponse{FollowerCount: count, IsFollowing: follow})
}

func (h *Handler) toJSON(id pgtype.UUID, username, title, category, thumbnailURL, avatarURL string, isLive bool, followerCount int64, isFollowing bool) channelJSON {
	count := 0
	if h.counter != nil {
		count = h.counter.Count(username)
	}
	return channelJSON{
		ID:            httputil.UUIDString(id),
		Username:      username,
		Title:         title,
		Category:      category,
		ThumbnailURL:  thumbnailURL,
		AvatarURL:     avatarURL,
		IsLive:        isLive,
		ViewerCount:   count,
		FollowerCount: followerCount,
		IsFollowing:   isFollowing,
	}
}

// resolveOptionalUser looks up the caller's session without requiring one —
// GET endpoints stay public, but authenticated viewers get is_following.
func (h *Handler) resolveOptionalUser(r *http.Request) *auth.SessionUser {
	c, err := r.Cookie(auth.SessionCookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	user, err := auth.ResolveSession(r.Context(), h.queries, c.Value)
	if err != nil {
		return nil
	}
	return user
}

func (h *Handler) isFollowing(r *http.Request, viewer *auth.SessionUser, channelID pgtype.UUID) bool {
	if viewer == nil {
		return false
	}
	following, err := h.queries.IsFollowing(r.Context(), db.IsFollowingParams{
		FollowerUserID: httputil.ParseUUID(viewer.ID),
		ChannelID:      channelID,
	})
	if err != nil {
		return false
	}
	return following
}

// parseIntParam parses a non-negative int, falling back to def on anything
// invalid. max <= 0 means uncapped.
func parseIntParam(raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return def
	}
	if max > 0 && n > max {
		return max
	}
	return n
}
