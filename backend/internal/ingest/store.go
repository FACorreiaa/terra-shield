package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"shield/internal/store/eventsdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event is one normalized record from an adapter. Adapters fill every
// field from their source; the store owns ids, countries and timestamps
// of its own.
type Event struct {
	SourceEventID  string
	Type           string // events.type check values
	Kind           string // observed | warning | forecast | detection (ADR 3)
	Status         string // active | resolved
	Severity       string // derived, CAP-aligned (ADR 2)
	SourceSeverity string // the source's own value, verbatim; "" for none
	Title          string
	Summary        string
	Geometry       json.RawMessage // GeoJSON geometry, WGS84; Z is dropped
	StartedAt      *time.Time
	ObservedAt     time.Time
	UpdatedAt      time.Time // the source's own last-update time
	EndedAt        *time.Time
	URL            string // official page for the event (ADR 12)
	Attrs          map[string]any
}

// Store writes normalized events.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore builds a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Upsert writes events for one source in a single transaction and returns
// how many were inserted or changed. Unchanged events cost no writes;
// changed ones get their countries reassigned (ADR 4).
func (s *Store) Upsert(ctx context.Context, sourceID string, events []Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := eventsdb.New(tx)

	changed := 0
	for i := range events {
		e := &events[i]
		attrs, err := json.Marshal(nonNilAttrs(e.Attrs))
		if err != nil {
			return 0, fmt.Errorf("event %s attrs: %w", e.SourceEventID, err)
		}
		id, err := q.UpsertEvent(ctx, eventsdb.UpsertEventParams{
			SourceID:       sourceID,
			SourceEventID:  e.SourceEventID,
			Type:           e.Type,
			Kind:           e.Kind,
			Status:         e.Status,
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
			Attrs:          attrs,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue // unchanged
		}
		if err != nil {
			return 0, fmt.Errorf("upsert event %s/%s: %w", sourceID, e.SourceEventID, err)
		}
		if err := q.ClearEventCountries(ctx, id); err != nil {
			return 0, err
		}
		if err := q.AssignEventCountries(ctx, id); err != nil {
			return 0, fmt.Errorf("assign countries %s/%s: %w", sourceID, e.SourceEventID, err)
		}
		changed++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return changed, nil
}

// Delete removes events a source has retracted (e.g. a USGS "deleted"
// event: a false detection, not a resolved one).
func (s *Store) Delete(ctx context.Context, sourceID string, sourceEventIDs []string) (int, error) {
	if len(sourceEventIDs) == 0 {
		return 0, nil
	}
	n, err := eventsdb.New(s.pool).DeleteEvents(ctx, eventsdb.DeleteEventsParams{
		SourceID: sourceID, SourceEventIds: sourceEventIDs,
	})
	return int(n), err
}

func nonNilAttrs(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
