package domain

import "time"

// ChangeKind is how an event changed over a window ("what changed in
// 24h", BACKLOG queue item 2).
type ChangeKind string

// Change kinds, one per event, in this precedence: an event that arrived
// and ended inside the window ended.
const (
	ChangeEnded      ChangeKind = "ended"      // resolved, and ended in the window
	ChangeNew        ChangeKind = "new"        // first seen active (or listed again) in the window
	ChangeEscalated  ChangeKind = "escalated"  // more severe than at the window's start
	ChangeDowngraded ChangeKind = "downgraded" // less severe than at the window's start
)

// ChangeKinds lists every kind, for counts.
func ChangeKinds() []ChangeKind {
	return []ChangeKind{ChangeNew, ChangeEscalated, ChangeDowngraded, ChangeEnded}
}

// Change is one event's net change over a window.
type Change struct {
	EventID        string
	SourceID       string
	Type           EventType
	Kind           Kind
	Severity       string // now
	FromSeverity   string // at the window's start; escalated and downgraded only
	SourceSeverity string
	Title          string
	Centroid       [2]float64 // lon, lat
	Countries      []string   // ISO2, every relation
	StartedAt      *time.Time
	Change         ChangeKind
	At             time.Time // when it arrived, ended, or last changed severity
}

// ChangeQuery filters a window of changes. Types is never empty
// (params.Types gives every hazard type by default).
type ChangeQuery struct {
	Since      time.Time
	Types      []EventType
	Severities []string // current severity; empty means any
	Country    string   // ISO2; "" means any
}
