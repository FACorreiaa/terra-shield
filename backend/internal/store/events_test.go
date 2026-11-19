package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedWorld loads two square "countries" and a source.
func seedWorld(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, sql := range []string{
		`INSERT INTO countries (iso2, iso3, name, geom) VALUES
		 ('PT', 'PRT', 'Portugal', ST_Multi('POLYGON((-9.5 37,-6.2 37,-6.2 42.1,-9.5 42.1,-9.5 37))'::geometry)::geography),
		 ('ES', 'ESP', 'Spain',    ST_Multi('POLYGON((-6.2 36,3.3 36,3.3 43.8,-6.2 43.8,-6.2 36))'::geometry)::geography)`,
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.NewSourceRepository(pool).Register(ctx, domain.SourceInfo{
		ID: "test", Name: "Test", URL: "https://example.org", License: "PD",
		Attribution: "Test", PollInterval: time.Minute, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func quake(id string, lon, lat, mag float64, updated time.Time) domain.Event {
	geom, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": []float64{lon, lat, 10}})
	return domain.Event{
		SourceEventID: id, Type: domain.TypeEarthquake, Kind: domain.KindObserved, Status: domain.StatusActive,
		Severity: "minor", SourceSeverity: "M", Title: id, Geometry: geom,
		ObservedAt: updated, UpdatedAt: updated, Attrs: map[string]any{"mag": mag},
	}
}

type countryRow struct {
	ISO2, Relation string
	DistanceKm     float32
}

func countriesOf(t *testing.T, pool *pgxpool.Pool, sourceEventID string) []countryRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT ec.iso2, ec.relation, ec.distance_km FROM event_countries ec
		JOIN events e ON e.id = ec.event_id
		WHERE e.source_event_id = $1 ORDER BY ec.iso2`, sourceEventID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []countryRow
	for rows.Next() {
		var r countryRow
		if err := rows.Scan(&r.ISO2, &r.Relation, &r.DistanceKm); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestEventUpsertAssignsCountries(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	s := store.NewEventRepository(pool)
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

	borderLine, _ := json.Marshal(map[string]any{"type": "LineString", "coordinates": [][]float64{{-8, 40}, {-5, 40}}})
	line := quake("border", 0, 0, 3, now)
	line.Geometry = borderLine

	n, err := s.Upsert(ctx, "test", []domain.Event{
		quake("inland", -8.0, 39.5, 3.1, now),    // inside PT
		quake("offshore", -10.5, 36.5, 4.2, now), // ~150 km off PT
		quake("ocean", -30, 30, 5.0, now),        // mid-Atlantic
		line,                                     // crosses PT/ES
	})
	if err != nil || n != 4 {
		t.Fatalf("upserted %d, err %v", n, err)
	}

	if got := countriesOf(t, pool, "inland"); len(got) != 1 || got[0] != (countryRow{"PT", "within", 0}) {
		t.Errorf("inland %v", got)
	}
	off := countriesOf(t, pool, "offshore")
	if len(off) != 1 || off[0].ISO2 != "PT" || off[0].Relation != "nearby" || off[0].DistanceKm < 50 || off[0].DistanceKm > 250 {
		t.Errorf("offshore %v", off)
	}
	if got := countriesOf(t, pool, "ocean"); len(got) != 0 {
		t.Errorf("ocean %v, want none beyond 250 km", got)
	}
	if got := countriesOf(t, pool, "border"); len(got) != 2 || got[0].ISO2 != "ES" || got[1].ISO2 != "PT" {
		t.Errorf("border %v", got)
	}
}

func TestEventUpsertSkipsUnchangedAndUpdatesChanged(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	s := store.NewEventRepository(pool)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

	if n, err := s.Upsert(ctx, "test", []domain.Event{quake("q", -8, 39.5, 3.1, t0)}); err != nil || n != 1 {
		t.Fatalf("first %d %v", n, err)
	}
	if n, err := s.Upsert(ctx, "test", []domain.Event{quake("q", -8, 39.5, 3.1, t0)}); err != nil || n != 0 {
		t.Fatalf("identical re-poll changed %d rows (err %v), want 0", n, err)
	}
	// The source revises the magnitude and relocates the epicenter into ES.
	revised := quake("q", -4, 40, 3.4, t0.Add(5*time.Minute))
	if n, err := s.Upsert(ctx, "test", []domain.Event{revised}); err != nil || n != 1 {
		t.Fatalf("revision %d %v", n, err)
	}
	if got := countriesOf(t, pool, "q"); len(got) != 1 || got[0].ISO2 != "ES" {
		t.Fatalf("countries after relocation %v", got)
	}
	var count int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM events`).Scan(&count)
	if count != 1 {
		t.Fatalf("%d events, want 1", count)
	}
}

func TestEventDelete(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	s := store.NewEventRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := s.Upsert(ctx, "test", []domain.Event{quake("a", -8, 39.5, 3, now), quake("b", -8, 39.6, 3, now)}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Delete(ctx, "test", []string{"a", "missing"}); err != nil || n != 1 {
		t.Fatalf("deleted %d %v", n, err)
	}
	if got := countriesOf(t, pool, "a"); len(got) != 0 {
		t.Fatal("event_countries not cascaded")
	}
}

func TestCountryAssignmentRules(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	s := store.NewEventRepository(pool)
	now := time.Now().UTC().Truncate(time.Second)
	box := func(id string, minLon, maxLon float64, rule domain.CountryRule) domain.Event {
		e := quake(id, 0, 0, 3, now)
		e.Geometry = json.RawMessage(fmt.Sprintf(
			`{"type":"Polygon","coordinates":[[[%[1]g,39],[%[2]g,39],[%[2]g,39.3],[%[1]g,39.3],[%[1]g,39]]]}`, minLon, maxLon))
		e.CountryRule = rule
		return e
	}
	pt := domain.CountryRule{Issuer: "PT", MinShare: 0.05}
	// PT/ES border at -6.2. "edge" reaches ~1 km into Spain: 0.5% of its area.
	if _, err := s.Upsert(context.Background(), "test", []domain.Event{
		box("plain", -8, -6.19, domain.CountryRule{}), // no rule: every country touched
		box("edge", -8, -6.19, pt),                    // sliver dropped
		box("spans", -7, -5, pt),                      // ~40% in Spain: kept
		box("offshore", -11, -10, pt),                 // touches nothing: issuer still assigned
		box("wraps", 179, -179, domain.CountryRule{}), // across the antimeridian, ~170 km wide
		// Named countries win over geometry: a forecast's marker sits at
		// the epicenter, far from the coasts it lists. Unknown codes drop.
		box("listed", -11, -10, domain.CountryRule{Listed: []string{"ES", "ZZ"}}),
	}); err != nil {
		t.Fatal(err)
	}
	iso := func(id string) string {
		var out []string
		for _, r := range countriesOf(t, pool, id) {
			out = append(out, r.ISO2+":"+r.Relation)
		}
		return fmt.Sprint(out)
	}
	for id, want := range map[string]string{
		"plain":    "[ES:within PT:within]",
		"edge":     "[PT:within]",
		"spans":    "[ES:within PT:within]",
		"offshore": "[PT:within]",
		"wraps":    "[]", // planar would wrap the long way and match PT and ES
		"listed":   "[ES:within]",
	} {
		if got := iso(id); got != want {
			t.Errorf("%s: %s, want %s", id, got, want)
		}
	}
}
