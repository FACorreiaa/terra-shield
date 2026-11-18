package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"

	"github.com/google/uuid"
)

func eventID(t *testing.T, repo *store.EventRepository, sourceEventID string) uuid.UUID {
	t.Helper()
	recs, _, err := repo.List(context.Background(), domain.EventQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for i := range recs {
		if recs[i].SourceEventID == sourceEventID {
			return uuid.MustParse(recs[i].ID)
		}
	}
	t.Fatalf("%s not stored", sourceEventID)
	return uuid.Nil
}

func TestRevisionsAndDetail(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	upsert := func(e domain.Event) {
		t.Helper()
		if _, err := repo.Upsert(ctx, "test", []domain.Event{e}); err != nil {
			t.Fatal(err)
		}
	}

	e := quake("q1", -8, 39, 5, t0)
	upsert(e) // created: not a revision
	upsert(e) // unchanged: nothing written

	e.Attrs = map[string]any{"mag": 5.1}
	e.UpdatedAt = t0.Add(time.Minute)
	upsert(e) // only attrs and timestamps: stored, but no revision

	e.Severity, e.SourceSeverity = "severe", "M 6.1"
	e.UpdatedAt = t0.Add(2 * time.Minute)
	upsert(e) // revision 1

	ended := t0.Add(time.Hour)
	e.Status, e.EndedAt = domain.StatusResolved, &ended
	e.Geometry, _ = json.Marshal(map[string]any{"type": "Point", "coordinates": []float64{-8.5, 39}})
	upsert(e) // revision 2

	id := eventID(t, repo, "q1")
	d, err := repo.Detail(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != id.String() || d.Severity != "severe" || d.Status != domain.StatusResolved || d.Attrs["mag"] != 5.1 {
		t.Fatalf("record %+v", d.EventRecord)
	}
	var g struct {
		Type        string
		Coordinates []float64
	}
	if err := json.Unmarshal(d.Geometry, &g); err != nil || g.Type != "Point" || g.Coordinates[0] != -8.5 {
		t.Fatalf("geometry %s err %v", d.Geometry, err)
	}
	if d.Source.ID != "test" || d.Source.PollInterval != time.Minute || !d.Source.Enabled || d.Source.Attribution != "Test" {
		t.Fatalf("source %+v", d.Source)
	}
	if len(d.Revisions) != 2 {
		t.Fatalf("%d revisions, want 2: %+v", len(d.Revisions), d.Revisions)
	}
	newest, first := d.Revisions[0].Changes, d.Revisions[1].Changes
	if _, ok := newest["status"]; !ok || newest["geometry"] == nil || newest["endedAt"] == nil || newest["severity"] != nil {
		t.Errorf("newest revision %v", newest)
	}
	sev := first["severity"].(map[string]any)
	if sev["from"] != "minor" || sev["to"] != "severe" || first["sourceSeverity"].(map[string]any)["to"] != "M 6.1" || len(first) != 2 {
		t.Errorf("first revision %v", first)
	}

	if _, err := repo.Detail(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing id: %v", err)
	}
}
