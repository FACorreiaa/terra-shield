// Package retention bounds what the database keeps (ADR 26): resolved
// events 90 days after they end (domain.ResolvedRetention), with their
// revisions and countries, and stream tombstones for TombstoneRetention.
// Active events are never purged, however old.
package retention

import (
	"context"
	"log/slog"
	"time"

	"shield/internal/domain"
	"shield/internal/metrics"
)

const (
	// TombstoneRetention: a stream client offline longer than this
	// reloads instead of resuming (it would need over 1,000 changes by
	// then anyway).
	TombstoneRetention = 7 * 24 * time.Hour
	// batch stays under a stream client's 256-change queue, so a purge
	// never overflows one.
	batch = 200
	// maxBatches and pause bound one run to about half a minute, inside
	// River's one-minute job timeout; a backlog drains over several runs.
	maxBatches = 20
	pause      = time.Second
	interval   = time.Hour
)

// Store is what the task deletes through.
type Store interface {
	PurgeResolved(ctx context.Context, before time.Time, limit int) (int, error)
	PruneTombstones(ctx context.Context, before time.Time) (int, error)
}

// Task is the hourly retention pass; it implements ingest.Task.
type Task struct {
	store  Store
	logger *slog.Logger
	now    func() time.Time
	pause  time.Duration
}

// New builds the task.
func New(store Store, logger *slog.Logger) *Task {
	return &Task{store: store, logger: logger, now: time.Now, pause: pause}
}

// Name implements ingest.Task.
func (*Task) Name() string { return "retention" }

// Interval implements ingest.Task.
func (*Task) Interval() time.Duration { return interval }

// Run deletes resolved events past retention in batches, then prunes old
// tombstones.
func (t *Task) Run(ctx context.Context) error {
	now := t.now()
	events := 0
	for i := range maxBatches {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(t.pause):
			}
		}
		n, err := t.store.PurgeResolved(ctx, now.Add(-domain.ResolvedRetention), batch)
		events += n
		metrics.RetentionDeleted.WithLabelValues("event").Add(float64(n))
		if err != nil {
			return err
		}
		if n < batch {
			break
		}
	}
	tombstones, err := t.store.PruneTombstones(ctx, now.Add(-TombstoneRetention))
	if err != nil {
		return err
	}
	metrics.RetentionDeleted.WithLabelValues("tombstone").Add(float64(tombstones))
	if events > 0 || tombstones > 0 {
		t.logger.Info("retention", "events", events, "tombstones", tombstones)
	}
	return nil
}
