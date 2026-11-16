// Package ingest runs the feed adapters on River schedules and records each
// source's health.
//
// Each adapter (internal/sources/<name>, from M1) implements Source. The
// worker registers one River periodic job per enabled source; a poll job
// loads the source's HTTP validators, calls Poll, and records success or
// failure on the sources row and in metrics.
package ingest

import (
	"context"
	"time"
)

// Info is a source's static metadata, written to the sources table on worker
// start so /v1/sources and the attribution screen always match the code.
type Info struct {
	ID           string // stable key: "usgs", "gdacs", ...
	Name         string
	URL          string // human-facing homepage of the feed
	License      string
	Attribution  string
	PollInterval time.Duration
	Enabled      bool
}

// Validators are the HTTP cache validators from the last successful poll,
// sent back as If-None-Match / If-Modified-Since.
type Validators struct {
	ETag         string
	LastModified string
}

// Result is what one poll did.
type Result struct {
	Validators  Validators // to keep for the next poll
	NotModified bool       // the feed answered 304
	Upserted    int        // events inserted or updated
}

// Source is one feed adapter. Poll fetches the feed once, normalizes and
// stores what changed, and must not retry on its own: River retries the job.
type Source interface {
	Info() Info
	Poll(ctx context.Context, v Validators) (Result, error)
}
