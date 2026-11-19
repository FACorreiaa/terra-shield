package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"shield/internal/domain"
	"shield/internal/store/eventsdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventRepository writes normalized events.
type EventRepository struct {
	pool *pgxpool.Pool
}

// NewEventRepository builds an EventRepository.
func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

// Upsert writes one source's events in a single transaction and returns how
// many were inserted or changed. Unchanged events cost no writes; changed
// ones get their countries reassigned (ADR 4), move the stream cursor and
// notify the API pods at commit.
func (r *EventRepository) Upsert(ctx context.Context, sourceID string, events []domain.Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := eventsdb.New(tx)
	if err := q.LockEventChanges(ctx); err != nil {
		return 0, fmt.Errorf("lock event changes: %w", err)
	}

	changed := 0
	for i := range events {
		ok, err := upsertOne(ctx, q, sourceID, &events[i])
		if err != nil {
			return 0, err
		}
		if ok {
			changed++
		}
	}
	if changed > 0 {
		if err := q.NotifyEventChanges(ctx); err != nil {
			return 0, fmt.Errorf("notify event changes: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return changed, nil
}

func upsertOne(ctx context.Context, q *eventsdb.Queries, sourceID string, e *domain.Event) (bool, error) {
	attrs := e.Attrs
	if attrs == nil {
		attrs = map[string]any{}
	}
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		return false, fmt.Errorf("event %s/%s attrs: %w", sourceID, e.SourceEventID, err)
	}
	key := eventsdb.EventSnapshotParams{SourceID: sourceID, SourceEventID: e.SourceEventID}
	before, err := q.EventSnapshot(ctx, key)
	existed := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("snapshot event %s/%s: %w", sourceID, e.SourceEventID, err)
	}

	id, err := q.UpsertEvent(ctx, eventsdb.UpsertEventParams{
		SourceID:       sourceID,
		SourceEventID:  e.SourceEventID,
		Type:           string(e.Type),
		Kind:           string(e.Kind),
		Status:         string(e.Status),
		Severity:       e.Severity,
		SourceSeverity: nonEmpty(e.SourceSeverity),
		Title:          e.Title,
		Summary:        nonEmpty(e.Summary),
		Geom:           string(e.Geometry),
		StartedAt:      e.StartedAt,
		ObservedAt:     e.ObservedAt,
		UpdatedAt:      e.UpdatedAt,
		EndedAt:        e.EndedAt,
		Url:            nonEmpty(e.URL),
		Attrs:          attrsJSON,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // unchanged
	}
	if err != nil {
		return false, fmt.Errorf("upsert event %s/%s: %w", sourceID, e.SourceEventID, err)
	}
	if existed {
		if err := recordRevision(ctx, q, key, &before); err != nil {
			return false, fmt.Errorf("revision %s/%s: %w", sourceID, e.SourceEventID, err)
		}
	}
	if err := q.ClearEventCountries(ctx, id); err != nil {
		return false, err
	}
	if err := q.AssignEventCountries(ctx, eventsdb.AssignEventCountriesParams{
		EventID: id, Issuer: e.CountryRule.Issuer, MinShare: e.CountryRule.MinShare,
	}); err != nil {
		return false, fmt.Errorf("assign countries %s/%s: %w", sourceID, e.SourceEventID, err)
	}
	return true, nil
}

// ActiveIDsWithAttr lists the source event ids of a source's active events
// whose attrs[key] equals value.
func (r *EventRepository) ActiveIDsWithAttr(ctx context.Context, sourceID, key, value string) ([]string, error) {
	return eventsdb.New(r.pool).ActiveEventIDsWithAttr(ctx, eventsdb.ActiveEventIDsWithAttrParams{
		SourceID: sourceID, AttrKey: key, AttrValue: value,
	})
}

// Resolve ends active events their source no longer lists, at the time
// given. Each gets a revision and moves the stream cursor, as an update
// would; events already resolved or unknown are skipped. It returns how
// many changed.
func (r *EventRepository) Resolve(ctx context.Context, sourceID string, sourceEventIDs []string, at time.Time) (int, error) {
	if len(sourceEventIDs) == 0 {
		return 0, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := eventsdb.New(tx)
	if err := q.LockEventChanges(ctx); err != nil {
		return 0, fmt.Errorf("lock event changes: %w", err)
	}
	changed := 0
	for _, id := range sourceEventIDs {
		key := eventsdb.EventSnapshotParams{SourceID: sourceID, SourceEventID: id}
		before, err := q.EventSnapshot(ctx, key)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("snapshot event %s/%s: %w", sourceID, id, err)
		}
		n, err := q.ResolveEvent(ctx, eventsdb.ResolveEventParams{SourceID: sourceID, SourceEventID: id, EndedAt: &at})
		if err != nil {
			return 0, fmt.Errorf("resolve event %s/%s: %w", sourceID, id, err)
		}
		if n == 0 {
			continue
		}
		if err := recordRevision(ctx, q, key, &before); err != nil {
			return 0, fmt.Errorf("revision %s/%s: %w", sourceID, id, err)
		}
		changed++
	}
	if changed > 0 {
		if err := q.NotifyEventChanges(ctx); err != nil {
			return 0, fmt.Errorf("notify event changes: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return changed, nil
}

// Delete removes events a source has retracted (a USGS "deleted" event is a
// false detection, not a resolved one). Each leaves a tombstone so stream
// clients drop it too.
func (r *EventRepository) Delete(ctx context.Context, sourceID string, sourceEventIDs []string) (int, error) {
	if len(sourceEventIDs) == 0 {
		return 0, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := eventsdb.New(tx)
	if err := q.LockEventChanges(ctx); err != nil {
		return 0, fmt.Errorf("lock event changes: %w", err)
	}
	ids, err := q.DeleteEvents(ctx, eventsdb.DeleteEventsParams{SourceID: sourceID, SourceEventIds: sourceEventIDs})
	if err != nil {
		return 0, fmt.Errorf("delete events: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if err := q.InsertEventTombstones(ctx, ids); err != nil {
		return 0, fmt.Errorf("event tombstones: %w", err)
	}
	if err := q.NotifyEventChanges(ctx); err != nil {
		return 0, fmt.Errorf("notify event changes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(ids), nil
}

// recordRevision writes what changed in an existing event, if a user
// would notice it.
func recordRevision(ctx context.Context, q *eventsdb.Queries, key eventsdb.EventSnapshotParams, before *eventsdb.EventSnapshotRow) error {
	after, err := q.EventSnapshot(ctx, key)
	if err != nil {
		return err
	}
	changes := revisionChanges(before, &after)
	if len(changes) == 0 {
		return nil
	}
	body, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	return q.InsertEventRevision(ctx, eventsdb.InsertEventRevisionParams{EventID: after.ID, Changes: body})
}
