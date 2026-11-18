package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

func TestCountryStatsAndSummary(t *testing.T) {
	pool := testutil.DB(t)
	events := seedEvents(t, pool) // PT: quake-pt (moderate), quake-off-pt (minor, nearby), resolved-pt; ES: drought-es (severe)
	ctx := context.Background()
	sources := store.NewSourceRepository(pool)
	countries := store.NewCountryRepository(pool)

	// A warning feed that covers Portugal only; "XX" is not a loaded country.
	if err := sources.Register(ctx, domain.SourceInfo{
		ID: "warn", Name: "Warn", URL: "https://example.org", License: "CC BY", Attribution: "Warn",
		PollInterval: time.Minute, Enabled: true, Coverage: []string{"PT", "XX"}, OfficialWarnings: true,
	}); err != nil {
		t.Fatal(err)
	}
	// One revision on a Portuguese event.
	e := quake("quake-pt", -8, 39, 5, listNow)
	e.Severity = "severe"
	if _, err := events.Upsert(ctx, "test", []domain.Event{e}); err != nil {
		t.Fatal(err)
	}

	all, err := countries.List(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("%+v %v", all, err)
	}
	es, pt := all[1], all[0] // by name: Portugal, Spain
	if pt.ISO2 != "PT" || pt.ActiveEvents != 2 || pt.MaxSeverity != "severe" || pt.BBox != [4]float64{-9.5, 37, -6.2, 42.1} {
		t.Errorf("PT %+v", pt)
	}
	if es.ISO2 != "ES" || es.ActiveEvents != 1 || es.MaxSeverity != "severe" || es.ISO3 != "ESP" {
		t.Errorf("ES %+v", es)
	}

	s, err := countries.Summary(ctx, "PT", listNow.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if s.Country.Name != "Portugal" || len(s.Events) != 2 || s.Events[0].SourceEventID != "quake-pt" {
		t.Fatalf("summary %+v events %v", s.Country, ids(s.Events))
	}
	if !slices.Equal(s.Counts, []domain.SeverityCount{
		{Type: domain.TypeEarthquake, Severity: "severe", Count: 1},
		{Type: domain.TypeEarthquake, Severity: "minor", Count: 1},
	}) {
		t.Errorf("counts %+v", s.Counts)
	}
	if !s.Coverage.OfficialWarnings || !slices.Equal(s.Coverage.Sources, []string{"test", "warn"}) {
		t.Errorf("PT coverage %+v", s.Coverage)
	}
	if len(s.RecentChanges) != 1 || s.RecentChanges[0].Title != "quake-pt" || s.RecentChanges[0].Changes["severity"] == nil {
		t.Errorf("changes %+v", s.RecentChanges)
	}

	es2, err := countries.Summary(ctx, "ES", time.Now().Add(time.Hour)) // window in the future: no changes
	if err != nil {
		t.Fatal(err)
	}
	if es2.Coverage.OfficialWarnings || !slices.Equal(es2.Coverage.Sources, []string{"test"}) || len(es2.RecentChanges) != 0 || len(es2.Events) != 1 {
		t.Errorf("ES summary %+v", es2)
	}

	// Re-registering replaces coverage: Spain now, Portugal no longer.
	if err := sources.Register(ctx, domain.SourceInfo{
		ID: "warn", Name: "Warn", URL: "https://example.org", License: "CC BY", Attribution: "Warn",
		PollInterval: time.Minute, Enabled: true, Coverage: []string{"ES"}, OfficialWarnings: true,
	}); err != nil {
		t.Fatal(err)
	}
	if s, _ := countries.Summary(ctx, "PT", listNow); s.Coverage.OfficialWarnings {
		t.Error("stale coverage kept after re-register")
	}

	if _, err := countries.Summary(ctx, "ZZ", listNow); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown country: %v", err)
	}
}
