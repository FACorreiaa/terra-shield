package sources

import (
	"testing"
	"time"

	"shield/internal/store/sourcesdb"
)

func TestStale(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }

	cases := []struct {
		name    string
		enabled bool
		last    *time.Time
		want    bool
	}{
		{"never polled", true, nil, true},
		{"fresh", true, ago(2 * time.Minute), false},
		{"three intervals is not yet stale", true, ago(3 * time.Minute), false},
		{"past three intervals", true, ago(4 * time.Minute), true},
		{"disabled is never stale", false, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toWire([]sourcesdb.ListSourcesRow{{
				ID: "x", PollIntervalSeconds: 60, Enabled: c.enabled, LastSuccessAt: c.last,
			}}, now)
			if got[0].Stale != c.want {
				t.Fatalf("stale = %v, want %v", got[0].Stale, c.want)
			}
		})
	}
}
