package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

func TestPurgeResolved(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	old, recent, oldActive := quake("old", -8, 39, 5, now), quake("recent", -8, 39, 5, now), quake("old-active", -8, 39, 5, now)
	old.Status, recent.Status = domain.StatusResolved, domain.StatusResolved
	if _, err := repo.Upsert(ctx, "test", []domain.Event{old, recent, oldActive}); err != nil {
		t.Fatal(err)
	}
	old.Severity = "moderate" // a revision, deleted with the event
	if _, err := repo.Upsert(ctx, "test", []domain.Event{old}); err != nil {
		t.Fatal(err)
	}
	oldID := eventID(t, repo, "old")
	for id, ago := range map[string]string{"old": "100 days", "recent": "10 days", "old-active": "200 days"} {
		if _, err := pool.Exec(ctx, `UPDATE events SET ended_at = now() - $2::interval, updated_at = now() - $2::interval
			WHERE source_event_id = $1`, id, ago); err != nil {
			t.Fatal(err)
		}
	}
	before, err := repo.LatestChange(ctx)
	if err != nil {
		t.Fatal(err)
	}

	n, err := repo.PurgeResolved(ctx, now.Add(-domain.ResolvedRetention), 200)
	if err != nil || n != 1 {
		t.Fatalf("purged %d err %v, want only the resolved event past 90 days", n, err)
	}
	var left, revisions int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM events), (SELECT count(*) FROM event_revisions WHERE event_id = $1)`, oldID).Scan(&left, &revisions); err != nil {
		t.Fatal(err)
	}
	if left != 2 || revisions != 0 {
		t.Errorf("%d events left (want recent + active), %d revisions of the purged one", left, revisions)
	}
	// Stream clients learn of it as a removal.
	changes, _, err := repo.Changes(ctx, before, 10)
	if err != nil || len(changes) != 1 || changes[0].Op != domain.OpRemoved || changes[0].EventID != oldID.String() {
		t.Errorf("changes %+v err %v, want the removal", changes, err)
	}

	// A feed still listing it does not bring it back: a new resolved event
	// past retention is not inserted.
	ended := now.Add(-100 * 24 * time.Hour)
	old.EndedAt, old.UpdatedAt = &ended, ended
	if n, err := repo.Upsert(ctx, "test", []domain.Event{old}); err != nil || n != 0 {
		t.Errorf("re-listed purged event: upserted %d err %v", n, err)
	}
	// An event still stored is updated as usual, however old.
	oa := quake("old-active", -8, 39, 6, now)
	if n, err := repo.Upsert(ctx, "test", []domain.Event{oa}); err != nil || n != 1 {
		t.Errorf("stored event: upserted %d err %v", n, err)
	}
}

func TestPruneTombstones(t *testing.T) {
	pool := testutil.DB(t)
	seedWorld(t, pool)
	repo := store.NewEventRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	if _, err := repo.Upsert(ctx, "test", []domain.Event{quake("a", -8, 39, 5, now), quake("b", -8, 39, 5, now)}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Delete(ctx, "test", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	var seq int64
	if err := pool.QueryRow(ctx, `UPDATE event_tombstones SET removed_at = now() - interval '8 days' RETURNING seq`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	latest, _ := repo.LatestChange(ctx)

	n, err := repo.PruneTombstones(ctx, now.Add(-7*24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("pruned %d err %v", n, err)
	}
	// A cursor that would have needed the pruned tombstone cannot resume.
	if _, _, err := repo.Changes(ctx, seq-1, 10); !errors.Is(err, domain.ErrCursorTooOld) {
		t.Errorf("cursor below the floor: err %v, want ErrCursorTooOld", err)
	}
	if _, _, err := repo.Changes(ctx, seq, 10); err != nil {
		t.Errorf("cursor at the floor: %v", err)
	}
	// The latest cursor does not move back though the tombstone held it.
	if got, err := repo.LatestChange(ctx); err != nil || got != latest {
		t.Errorf("latest %d err %v, want %d", got, err, latest)
	}
	if n, err := repo.PruneTombstones(ctx, now.Add(-7*24*time.Hour)); err != nil || n != 0 {
		t.Errorf("second prune %d err %v", n, err)
	}
}
