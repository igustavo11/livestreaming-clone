package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/streamkey"
)

const reconcileInterval = 30 * time.Second

type pathResponse struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

type pathsListResponse struct {
	Items []pathResponse `json:"items"`
}

type Reconciler struct {
	queries        *db.Queries
	mediamtxURL    string
	internalSecret string
	httpClient     *http.Client
	logger         *slog.Logger
}

func NewReconciler(queries *db.Queries, mediamtxURL, internalSecret string, logger *slog.Logger) *Reconciler {
	return &Reconciler{
		queries:        queries,
		mediamtxURL:    mediamtxURL,
		internalSecret: internalSecret,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		logger:         logger,
	}
}

// Start runs the reconciler loop until the context is cancelled.
func (r *Reconciler) Start(ctx context.Context) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	r.logger.Info("reconciler started", "interval", reconcileInterval)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("reconciler stopped")
			return
		case <-ticker.C:
			if err := r.reconcile(ctx); err != nil {
				r.logger.Error("reconcile error", "error", err)
			}
		}
	}
}

func (r *Reconciler) reconcile(ctx context.Context) error {
	activePaths, err := r.fetchActivePaths(ctx)
	if err != nil {
		return fmt.Errorf("fetch active paths: %w", err)
	}

	// Build set of hashed active stream keys for O(1) lookup
	activeKeyHashes := make(map[string]bool)
	for _, p := range activePaths {
		if !p.Available {
			continue
		}
		key := extractKeyFromPath(p.Name)
		if key == "" {
			continue
		}
		activeKeyHashes[streamkey.Hash(key)] = true
	}

	// Get all channels that are currently marked live
	liveChannels, err := r.queries.GetAllLiveChannels(ctx)
	if err != nil {
		return fmt.Errorf("get live channels: %w", err)
	}

	// For each live channel, check if it has an active stream
	for _, ch := range liveChannels {
		if !activeKeyHashes[ch.StreamKeyHash.String] {
			// Channel is marked live but no active stream — mark offline
			if err := r.queries.SetChannelLive(ctx, db.SetChannelLiveParams{
				ID:     ch.ID,
				IsLive: false,
			}); err != nil {
				r.logger.Error("failed to set channel offline", "channel_id", ch.ID, "error", err)
				continue
			}
			r.logger.Info("reconciled channel offline", "channel_id", ch.ID)
		}
	}

	return nil
}

func (r *Reconciler) fetchActivePaths(ctx context.Context) ([]pathResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.mediamtxURL+"/v3/paths/list", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Secret", r.internalSecret)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mediamtx API returned %d", resp.StatusCode)
	}

	limitedBody := io.LimitReader(resp.Body, 10<<20)
	var result pathsListResponse
	if err := json.NewDecoder(limitedBody).Decode(&result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

// AuthenticatePath checks if a path (from MediaMTX) corresponds to a valid channel.
// Returns the channel ID if valid, empty UUID otherwise.
func (r *Reconciler) AuthenticatePath(ctx context.Context, path string) (pgtype.UUID, error) {
	key := extractKeyFromPath(path)
	if key == "" {
		return pgtype.UUID{}, nil
	}

	ch, err := r.queries.GetChannelByStreamKeyHash(ctx, pgtype.Text{String: streamkey.Hash(key), Valid: true})
	if err != nil {
		return pgtype.UUID{}, nil
	}

	return ch.ID, nil
}
