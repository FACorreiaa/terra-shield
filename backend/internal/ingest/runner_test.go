package ingest

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/metrics"

	promtest "github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSeedHealthExportsEverySourceBeforeItsFirstPoll(t *testing.T) {
	states := newFakeStates()
	polled := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	states.successAt["seed-polled"] = polled
	r := NewRunner(nil, states, nil, slog.New(slog.DiscardHandler))
	started := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	for _, id := range []string{"seed-polled", "seed-never"} {
		info := domain.SourceInfo{ID: id, PollInterval: 5 * time.Minute}
		if err := r.seedHealth(context.Background(), info, started); err != nil {
			t.Fatal(err)
		}
		if got := promtest.ToFloat64(metrics.SourcePollInterval.WithLabelValues(id)); got != 300 {
			t.Fatalf("%s interval %v, want 300", id, got)
		}
	}
	// Last success survives a restart; a source that never succeeded
	// counts from the start, so failing from boot still goes stale.
	if got := promtest.ToFloat64(metrics.SourceLastSuccess.WithLabelValues("seed-polled")); got != float64(polled.Unix()) {
		t.Fatalf("polled source seeded at %v, want %d", got, polled.Unix())
	}
	if got := promtest.ToFloat64(metrics.SourceLastSuccess.WithLabelValues("seed-never")); got != float64(started.Unix()) {
		t.Fatalf("never-polled source seeded at %v, want the start %d", got, started.Unix())
	}
}
