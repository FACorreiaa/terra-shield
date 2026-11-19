// Package ingest runs the feed adapters on River schedules and records each
// source's health.
//
// Each adapter (internal/sources/<name>) implements Source. The worker
// registers one River periodic job per enabled source; a poll job loads the
// source's HTTP validators, calls Poll, and records success or failure in
// the source state store and in metrics. Sources that keep rolling data
// may also implement Housekeeper for an hourly cleanup job.
//
// Everything this package needs from storage is declared here as an
// interface (SourceStates, EventStore, FireStore); internal/app wires in
// the repositories from internal/store.
package ingest

import (
	"context"
	"time"

	"shield/internal/domain"
)

// Result is what one poll did.
type Result struct {
	Validators  domain.Validators // to keep for the next poll
	NotModified bool              // the feed had nothing new
	Upserted    int               // records inserted, updated or deleted
	// SourceUpdatedAt is when the source last updated what was fetched,
	// for adapters that know it better than one Last-Modified header
	// (FIRMS reads several files). Zero: the worker uses Last-Modified.
	SourceUpdatedAt time.Time
}

// Source is one feed adapter. Poll fetches the feed once, normalizes and
// stores what changed, and must not retry on its own: River retries the job.
type Source interface {
	Info() domain.SourceInfo
	Poll(ctx context.Context, v domain.Validators) (Result, error)
}

// Housekeeper is a Source with periodic cleanup, such as purging a rolling
// window. Housekeep runs every HousekeepInterval.
type Housekeeper interface {
	Housekeep(ctx context.Context) error
	HousekeepInterval() time.Duration
}

// SlowSource is a Source whose poll legitimately needs longer than the
// default bound, such as one that reads dozens of feeds. PollTimeout must
// stay below its poll interval.
type SlowSource interface {
	PollTimeout() time.Duration
}

// SourceStates persists each source's metadata and polling state.
type SourceStates interface {
	Register(ctx context.Context, info domain.SourceInfo) error
	Validators(ctx context.Context, id string) (domain.Validators, error)
	LastSuccess(ctx context.Context, id string) (*time.Time, error)
	RecordSuccess(ctx context.Context, id string, at time.Time, v domain.Validators) error
	RecordError(ctx context.Context, id string, at time.Time, msg string) error
}

// EventStore is where event adapters write.
type EventStore interface {
	Upsert(ctx context.Context, sourceID string, events []domain.Event) (int, error)
	Delete(ctx context.Context, sourceID string, sourceEventIDs []string) (int, error)
}

// FireStore is where the FIRMS adapter writes.
type FireStore interface {
	Insert(ctx context.Context, fires []domain.FireDetection) (int, error)
	PurgeBefore(ctx context.Context, t time.Time) (int, error)
}
