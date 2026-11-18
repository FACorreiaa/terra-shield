package store_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

type opAt struct {
	id string
	op domain.ChangeOp
}

func changeOps(cs []domain.StreamChange) []opAt {
	out := make([]opAt, 0, len(cs))
	for _, c := range cs {
		id := c.EventID
		if c.Event != nil {
			id = c.Event.SourceEventID
		}
		out = append(out, opAt{id, c.Op})
	}
	return out
}

func TestEventChangeLog(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	if seq, err := repo.LatestChange(ctx); err != nil || seq != 0 {
		t.Fatalf("empty log at %d %v", seq, err)
	}
	if _, err := repo.Upsert(ctx, "test", []domain.Event{quake("a", -8, 39.5, 3, now), quake("b", -3, 40, 3, now)}); err != nil {
		t.Fatal(err)
	}
	start, _ := repo.LatestChange(ctx)

	all, more, err := repo.Changes(ctx, 0, 10)
	if err != nil || more || len(all) != 2 || all[0].Op != domain.OpCreated || all[1].Seq != start {
		t.Fatalf("%+v more=%v %v", changeOps(all), more, err)
	}
	if b := all[0].Box; b == nil || b.MinLon != -8 || b.MaxLat != 39.5 {
		t.Fatalf("box %+v", b)
	}
	if e := all[0].Event; e == nil || len(e.Countries) == 0 || e.Countries[0].ISO2 != "PT" {
		t.Fatalf("record %+v", e)
	}

	// Unchanged: no new cursor. Changed: updated. Resolved: resolved.
	_, _ = repo.Upsert(ctx, "test", []domain.Event{quake("a", -8, 39.5, 3, now)})
	if seq, _ := repo.LatestChange(ctx); seq != start {
		t.Fatalf("unchanged upsert moved the cursor %d → %d", start, seq)
	}
	a := quake("a", -8, 39.5, 3, now)
	a.Severity = "moderate"
	b := quake("b", -3, 40, 3, now)
	b.Status = domain.StatusResolved
	if _, err := repo.Upsert(ctx, "test", []domain.Event{a, b}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Delete(ctx, "test", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	got, more, err := repo.Changes(ctx, start, 10)
	want := []opAt{{"b", domain.OpResolved}, {all[0].EventID, domain.OpRemoved}}
	if err != nil || more || len(got) != 2 || changeOps(got)[0] != want[0] || changeOps(got)[1] != want[1] {
		t.Fatalf("after %d: %+v more=%v %v", start, changeOps(got), more, err)
	}
	if got[1].Event != nil || got[1].Box != nil {
		t.Fatalf("removal carries an event: %+v", got[1])
	}
	latest, _ := repo.LatestChange(ctx)
	if latest != got[1].Seq {
		t.Fatalf("latest %d, last change %d", latest, got[1].Seq)
	}

	// Paging across both tables keeps cursor order.
	page, more, err := repo.Changes(ctx, 0, 1)
	if err != nil || !more || len(page) != 1 || page[0].Op != domain.OpResolved {
		t.Fatalf("page 1 %+v more=%v %v", changeOps(page), more, err)
	}
	page, more, _ = repo.Changes(ctx, page[0].Seq, 1)
	if more || len(page) != 1 || page[0].Op != domain.OpRemoved {
		t.Fatalf("page 2 %+v more=%v", changeOps(page), more)
	}
}

func TestEventListenWakesOnCommit(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	woke := make(chan struct{}, 10)
	done := make(chan error, 1)
	go func() {
		done <- repo.Listen(ctx, slog.New(slog.DiscardHandler), func() { woke <- struct{}{} })
	}()
	wait := func(what string) {
		t.Helper()
		select {
		case <-woke:
		case <-time.After(5 * time.Second):
			t.Fatalf("no wake: %s", what)
		}
	}
	wait("on connect")

	now := time.Now().UTC()
	if _, err := repo.Upsert(ctx, "test", []domain.Event{quake("a", -8, 39.5, 3, now)}); err != nil {
		t.Fatal(err)
	}
	wait("after upsert")

	// An unchanged upsert does not notify.
	if _, err := repo.Upsert(ctx, "test", []domain.Event{quake("a", -8, 39.5, 3, now)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-woke:
		t.Fatal("woken by an unchanged upsert")
	case <-time.After(300 * time.Millisecond):
	}

	if _, err := repo.Delete(ctx, "test", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	wait("after delete")

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("listen returned %v", err)
	}
}
