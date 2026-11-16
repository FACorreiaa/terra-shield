package migrate_test

import (
	"context"
	"log/slog"
	"testing"

	"shield/internal/migrate"
	"shield/internal/testutil"
)

func TestSchema(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()

	var postgis string
	if err := pool.QueryRow(ctx, `SELECT postgis_lib_version()`).Scan(&postgis); err != nil {
		t.Fatalf("postgis: %v", err)
	}
	for _, table := range []string{
		"sources", "countries", "events", "event_countries", "event_revisions",
		"fire_detections", "source_coverage", "river_job",
	} {
		var ok bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&ok); err != nil || !ok {
			t.Errorf("table %s missing (err %v)", table, err)
		}
	}

	// Re-running is a no-op, as the PreSync job does on every rollout.
	if err := migrate.Up(ctx, pool, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatalf("second Up: %v", err)
	}
}

func TestFireDetectionPointIsDerived(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO fire_detections (satellite, acq_at, lat, lon) VALUES ('N20', now(), 38.7, -9.1)`)
	if err != nil {
		t.Fatal(err)
	}
	var x, y float64
	if err := pool.QueryRow(ctx, `SELECT ST_X(point::geometry), ST_Y(point::geometry) FROM fire_detections`).Scan(&x, &y); err != nil {
		t.Fatal(err)
	}
	if x != -9.1 || y != 38.7 {
		t.Fatalf("point (%v, %v), want (-9.1, 38.7)", x, y)
	}
}

func TestEventChecks(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO sources (id, name, url, license, attribution, poll_interval_seconds)
		VALUES ('usgs', 'USGS', 'https://earthquake.usgs.gov', 'Public domain', 'USGS', 60)`); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO events (source_id, source_event_id, type, kind, severity, title, geom, centroid, observed_at, updated_at)
		VALUES ('usgs', $1, 'earthquake', $2, 'moderate', 'M5.1', 'POINT(-9 38)', 'POINT(-9 38)', now(), now())`
	if _, err := pool.Exec(ctx, insert, "a", "observed"); err != nil {
		t.Fatalf("valid event: %v", err)
	}
	if _, err := pool.Exec(ctx, insert, "b", "rumor"); err == nil {
		t.Fatal("kind check accepted 'rumor'")
	}
	if _, err := pool.Exec(ctx, insert, "a", "observed"); err == nil {
		t.Fatal("duplicate (source_id, source_event_id) accepted")
	}
}
