package store_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

// KnownQuakes at its edges: time, distance and magnitude each just in and
// just out; a stored quake whose mag is not a number, or that is not an
// earthquake, or under another source, matches nothing.
func TestKnownQuakes(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	ctx := context.Background()
	events := store.NewEventRepository(pool)
	at := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	quake := func(id string, lon, lat float64, mag any, typ domain.EventType) domain.Event {
		g, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": []float64{lon, lat}})
		return domain.Event{
			SourceEventID: id, Type: typ, Kind: domain.KindObserved, Status: domain.StatusActive, Severity: "minor",
			Title: id, Geometry: g, ObservedAt: at, UpdatedAt: at, Attrs: map[string]any{"mag": mag},
		}
	}
	if _, err := events.Upsert(ctx, "test", []domain.Event{
		quake("q", 20, 40, 4.0, domain.TypeEarthquake),
		quake("text-mag", 30, 40, "4.0", domain.TypeEarthquake),
		quake("flood", 10, 40, 4.0, domain.TypeFlood),
	}); err != nil {
		t.Fatal(err)
	}
	m := domain.QuakeMatch{Within: 30 * time.Second, Meters: 100_000, MagDelta: 0.8}
	// 1° of latitude is about 111 km: 0.89° in, 0.91° out.
	cases := []struct {
		name string
		q    domain.QuakeOrigin
		want bool
	}{
		{"same", domain.QuakeOrigin{At: at, Lon: 20, Lat: 40, Mag: 4.0}, true},
		{"29 s later", domain.QuakeOrigin{At: at.Add(29 * time.Second), Lon: 20, Lat: 40, Mag: 4.0}, true},
		{"31 s earlier", domain.QuakeOrigin{At: at.Add(-31 * time.Second), Lon: 20, Lat: 40, Mag: 4.0}, false},
		{"99 km", domain.QuakeOrigin{At: at, Lon: 20, Lat: 40.89, Mag: 4.0}, true},
		{"101 km", domain.QuakeOrigin{At: at, Lon: 20, Lat: 40.91, Mag: 4.0}, false},
		{"0.75 smaller", domain.QuakeOrigin{At: at, Lon: 20, Lat: 40, Mag: 3.25}, true},
		{"0.85 larger", domain.QuakeOrigin{At: at, Lon: 20, Lat: 40, Mag: 4.85}, false},
		{"text mag", domain.QuakeOrigin{At: at, Lon: 30, Lat: 40, Mag: 4.0}, false},
		{"not a quake", domain.QuakeOrigin{At: at, Lon: 10, Lat: 40, Mag: 4.0}, false},
	}
	qs := make([]domain.QuakeOrigin, len(cases))
	for i := range cases {
		qs[i] = cases[i].q
	}
	got, err := events.KnownQuakes(ctx, "test", qs, m)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		if got[i] != c.want {
			t.Errorf("%s: %v, want %v", c.name, got[i], c.want)
		}
	}
	if other, err := events.KnownQuakes(ctx, "usgs", qs[:1], m); err != nil || slices.Contains(other, true) {
		t.Errorf("another source: %v, %v", other, err)
	}
	if none, err := events.KnownQuakes(ctx, "test", nil, m); err != nil || len(none) != 0 {
		t.Errorf("no quakes: %v, %v", none, err)
	}
}
