package domain

import (
	"errors"
	"time"
)

// ResolvedRetention is how long a resolved event is kept after it ends
// (owner, 2026-11-19; ADR 26). Its revisions and countries go with it.
const ResolvedRetention = 90 * 24 * time.Hour

// PastRetention reports whether e is a resolved event that ended before
// horizon: the retention purge deletes such events, so a source that
// still lists one must not insert it again.
func PastRetention(e *Event, horizon time.Time) bool {
	if e.Status != StatusResolved {
		return false
	}
	end := e.UpdatedAt
	if e.EndedAt != nil {
		end = *e.EndedAt
	}
	return end.Before(horizon)
}

// ErrCursorTooOld means a stream cursor is below the stream floor: the
// tombstones it would need were pruned, so the client must reload.
var ErrCursorTooOld = errors.New("stream cursor older than the retained change log")
