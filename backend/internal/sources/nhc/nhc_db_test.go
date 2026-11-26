package nhc

import (
	"context"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

// Against PostGIS: the sample list stores Gert and Franklin with their
// pins at the advisory's center; an unchanged list writes nothing; a
// storm gone from the list ends.
func TestPollStores(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	if err := store.NewSourceRepository(pool).Register(ctx, New(nil, nil, nil).Info()); err != nil {
		t.Fatal(err)
	}
	srv, _, set := stub(t)
	s := newSource(t, srv, store.NewEventRepository(pool))
	s.now = func() time.Time { return time.Date(2023, 8, 22, 4, 0, 0, 0, time.UTC) }

	poll := func() int {
		t.Helper()
		res, err := s.Poll(ctx, domain.Validators{})
		if err != nil {
			t.Fatal(err)
		}
		return res.Upserted
	}
	if n := poll(); n != 2 {
		t.Fatalf("first poll stored %d", n)
	}
	var lon, lat float64
	var valid bool
	var kind string
	if err := pool.QueryRow(ctx, `SELECT ST_X(centroid::geometry), ST_Y(centroid::geometry), ST_IsValid(geom::geometry), kind
		FROM events WHERE source_id = $1 AND source_event_id = 'al062023'`, ID).Scan(&lon, &lat, &valid, &kind); err != nil {
		t.Fatal(err)
	}
	if lon != -58.4 || lat != 17.1 || !valid || kind != "forecast" {
		t.Fatalf("Gert pinned at %v,%v, valid %v, kind %s", lon, lat, valid, kind)
	}
	if n := poll(); n != 0 {
		t.Fatalf("an unchanged list changed %d events", n)
	}
	set(`{"activeStorms": []}`)
	if n := poll(); n != 2 {
		t.Fatalf("an empty list ended %d", n)
	}
	var active int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE source_id = $1 AND status = 'active'`, ID).Scan(&active); err != nil || active != 0 {
		t.Fatalf("%d still active, %v", active, err)
	}
}

// Dora's cone, cut at the antimeridian, is stored as the area either side
// of it, not the long way round the world.
func TestAntimeridianConeStored(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	if err := store.NewSourceRepository(pool).Register(ctx, New(nil, nil, nil).Info()); err != nil {
		t.Fatal(err)
	}
	f, err := parseForecast(readFixture(t, "ep052023_5day_040.zip"), time.Date(2023, 8, 10, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	lat, lon := 12.1, -169.8
	ev, err := build(&storm{
		ID: "ep052023", Name: "Dora", Classification: "HU", Intensity: "120",
		LatitudeNumeric: &lat, LongitudeNumeric: &lon, LastUpdate: "2023-08-10T09:00:00.000Z",
		TrackCone: &product{AdvNum: "040", Issuance: "2023-08-10T09:00:00.000Z"},
	}, f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NewEventRepository(pool).Upsert(ctx, ID, []domain.Event{ev}); err != nil {
		t.Fatal(err)
	}
	var east, west, atlantic bool
	if err := pool.QueryRow(ctx, `SELECT
			ST_Intersects(geom, ST_MakePoint(178.2, 16.1)::geography),
			ST_Intersects(geom, ST_MakePoint(-175.8, 13.7)::geography),
			ST_Intersects(geom, ST_MakePoint(0, 18)::geography)
		FROM events WHERE source_id = $1`, ID).Scan(&east, &west, &atlantic); err != nil {
		t.Fatal(err)
	}
	if !east || !west || atlantic {
		t.Fatalf("cone covers the 48 h point (178.2E) %v, the 24 h point (175.8W) %v, 0E %v", east, west, atlantic)
	}
}
