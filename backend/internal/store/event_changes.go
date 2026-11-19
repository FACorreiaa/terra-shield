package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"shield/internal/domain"
	"shield/internal/store/eventsdb"

	"github.com/jackc/pgx/v5"
)

// eventChannel is the LISTEN/NOTIFY channel event writers signal on.
const eventChannel = "shield_events"

// Changes returns up to limit entries of the change log after cursor
// `after`, in cursor order, and whether more follow. Events come with
// their list-view record (no geometry) and bounding box; removals only
// with their id. Both tables are read in one snapshot, so a commit landing
// between the reads cannot advance a cursor past a change it missed. A
// cursor below the stream floor (pruned tombstones; migration 00008) is
// domain.ErrCursorTooOld, read in the same snapshot.
func (r *EventRepository) Changes(ctx context.Context, after int64, limit int) ([]domain.StreamChange, bool, error) {
	if limit <= 0 {
		return nil, false, fmt.Errorf("event changes: limit must be positive")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	floor, err := eventsdb.New(tx).StreamFloor(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("stream floor: %w", err)
	}
	if after < floor {
		return nil, false, domain.ErrCursorTooOld
	}
	rows, err := tx.Query(ctx, `
SELECT `+recordColumns+`,
       NULL::text,
       e.change_seq, e.change_op,
       ST_XMin(e.geom::geometry), ST_YMin(e.geom::geometry), ST_XMax(e.geom::geometry), ST_YMax(e.geom::geometry)
FROM events e
WHERE e.change_seq > $1
ORDER BY e.change_seq
LIMIT $2`, after, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("event changes: %w", err)
	}
	var updates []domain.StreamChange
	for rows.Next() {
		var (
			c  domain.StreamChange
			op string
			bb domain.BBox
		)
		rec, err := scanRecord(rows, &c.Seq, &op, &bb.MinLon, &bb.MinLat, &bb.MaxLon, &bb.MaxLat)
		if err != nil {
			rows.Close()
			return nil, false, err
		}
		c.Op, c.EventID, c.Event, c.Box = domain.ChangeOp(op), rec.ID, &rec, &bb
		updates = append(updates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("event changes: %w", err)
	}

	rows, err = tx.Query(ctx, `
SELECT seq, event_id::text FROM event_tombstones WHERE seq > $1 ORDER BY seq LIMIT $2`, after, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("event tombstones: %w", err)
	}
	var removals []domain.StreamChange
	for rows.Next() {
		c := domain.StreamChange{Op: domain.OpRemoved}
		if err := rows.Scan(&c.Seq, &c.EventID); err != nil {
			rows.Close()
			return nil, false, fmt.Errorf("event tombstones: %w", err)
		}
		removals = append(removals, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("event tombstones: %w", err)
	}
	return mergeChanges(updates, removals, limit)
}

// mergeChanges merges two cursor-ordered lists, each fetched with limit+1
// rows, into the first limit entries overall.
func mergeChanges(a, b []domain.StreamChange, limit int) ([]domain.StreamChange, bool, error) {
	out := make([]domain.StreamChange, 0, min(len(a)+len(b), limit))
	i, j := 0, 0
	for len(out) < limit && (i < len(a) || j < len(b)) {
		if j >= len(b) || (i < len(a) && a[i].Seq < b[j].Seq) {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	return out, i < len(a) || j < len(b), nil
}

// LatestChange is the highest committed change cursor (0 when none).
func (r *EventRepository) LatestChange(ctx context.Context) (int64, error) {
	seq, err := eventsdb.New(r.pool).LatestEventChange(ctx)
	if err != nil {
		return 0, fmt.Errorf("latest event change: %w", err)
	}
	return seq, nil
}

// Listen holds a dedicated connection LISTENing for event changes and calls
// wake on every notification, and once after each (re)connect so changes
// made while disconnected are caught up. It reconnects with backoff and
// returns when ctx ends.
func (r *EventRepository) Listen(ctx context.Context, logger *slog.Logger, wake func()) error {
	backoff := time.Second
	for {
		err := r.listenOnce(ctx, wake, func() { backoff = time.Second })
		if ctx.Err() != nil {
			return nil
		}
		logger.Warn("event listener disconnected; reconnecting", "error", err, "in", backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (r *EventRepository) listenOnce(ctx context.Context, wake, connected func()) error {
	conn, err := pgx.ConnectConfig(ctx, r.pool.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	if _, err := conn.Exec(ctx, "LISTEN "+eventChannel); err != nil {
		return err
	}
	connected()
	wake()
	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return nil
			}
			return err
		}
		wake()
	}
}
