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

	e.Title = "M 5.0 - renamed"
	e.UpdatedAt = t0.Add(90 * time.Second)
	upsert(e) // a rename alone: stored, but no revision

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
	if d.ID != id.String() || d.Title != "M 5.0 - renamed" || d.Severity != "severe" || d.Status != domain.StatusResolved || d.Attrs["mag"] != 5.1 {
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

func TestActiveIDsWithAttr(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	pt, es, gone := quake("pt", -8, 39, 5, t0), quake("es", -4, 40, 5, t0), quake("pt-old", -8, 39, 5, t0)
	pt.Attrs = map[string]any{"country": "PT"}
	es.Attrs = map[string]any{"country": "ES"}
	gone.Attrs = map[string]any{"country": "PT"}
	gone.Status = domain.StatusResolved
	if _, err := repo.Upsert(ctx, "test", []domain.Event{pt, es, gone}); err != nil {
		t.Fatal(err)
	}
	ids, err := repo.ActiveIDsWithAttr(ctx, "test", "country", "PT")
	if err != nil || len(ids) != 1 || ids[0] != "pt" {
		t.Fatalf("ids %v err %v, want only the active PT event", ids, err)
	}
}

func TestResolve(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	e := quake("w1", -8, 39, 5, t0)
	if _, err := repo.Upsert(ctx, "test", []domain.Event{e}); err != nil {
		t.Fatal(err)
	}
	var seqBefore int64
	if err := pool.QueryRow(ctx, "SELECT change_seq FROM events WHERE source_event_id = 'w1'").Scan(&seqBefore); err != nil {
		t.Fatal(err)
	}
	at := t0.Add(3 * time.Hour)
	n, err := repo.Resolve(ctx, "test", []string{"w1", "unknown"}, at)
	if err != nil || n != 1 {
		t.Fatalf("resolved %d err %v, want 1", n, err)
	}
	// Resolving again changes nothing.
	if n, err := repo.Resolve(ctx, "test", []string{"w1"}, at.Add(time.Hour)); err != nil || n != 0 {
		t.Fatalf("second resolve %d err %v", n, err)
	}

	d, err := repo.Detail(ctx, eventID(t, repo, "w1"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != domain.StatusResolved || d.EndedAt == nil || !d.EndedAt.Equal(at) || !d.UpdatedAt.Equal(e.UpdatedAt) {
		t.Fatalf("status %s endedAt %v updatedAt %v: want resolved at %v, source updatedAt kept", d.Status, d.EndedAt, d.UpdatedAt, at)
	}
	if len(d.Revisions) != 1 || d.Revisions[0].Changes["status"] == nil || d.Revisions[0].Changes["endedAt"] == nil {
		t.Fatalf("revisions %+v, want one with status and endedAt", d.Revisions)
	}
	var seqAfter int64
	var op string
	if err := pool.QueryRow(ctx, "SELECT change_seq, change_op FROM events WHERE source_event_id = 'w1'").Scan(&seqAfter, &op); err != nil {
		t.Fatal(err)
	}
	if seqAfter <= seqBefore || op != "resolved" {
		t.Fatalf("change_seq %d → %d op %s: the stream must see the resolve", seqBefore, seqAfter, op)
	}
}
