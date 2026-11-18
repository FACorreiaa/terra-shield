package store

import (
	"time"

	"shield/internal/store/eventsdb"
)

// revisionChanges diffs the tracked fields of an event before and after an
// upsert (ADR 6). Each changed field maps to {"from", "to"}; a geometry
// change is only flagged, since shapes can be hundreds of KB. An empty
// result means nothing a user would notice changed (only summary, attrs
// or timestamps), and no revision is written.
func revisionChanges(before, after *eventsdb.EventSnapshotRow) map[string]any {
	changes := map[string]any{}
	diff := func(key string, from, to any) {
		changes[key] = map[string]any{"from": from, "to": to}
	}
	if before.Severity != after.Severity {
		diff("severity", before.Severity, after.Severity)
	}
	if before.Status != after.Status {
		diff("status", before.Status, after.Status)
	}
	if deref(before.SourceSeverity) != deref(after.SourceSeverity) {
		diff("sourceSeverity", before.SourceSeverity, after.SourceSeverity)
	}
	if before.Title != after.Title {
		diff("title", before.Title, after.Title)
	}
	if !sameTime(before.EndedAt, after.EndedAt) {
		diff("endedAt", before.EndedAt, after.EndedAt)
	}
	if before.GeomHash != after.GeomHash {
		changes["geometry"] = map[string]any{"changed": true}
	}
	return changes
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
