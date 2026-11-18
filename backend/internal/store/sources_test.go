package store_test

import (
	"context"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

func TestSourceLifecycle(t *testing.T) {
	repo := store.NewSourceRepository(testutil.DB(t))
	ctx := context.Background()
	info := domain.SourceInfo{
		ID: "s", Name: "S", URL: "https://example.org", License: "PD",
		Attribution: "S", PollInterval: time.Minute, Enabled: true,
	}
	if err := repo.Register(ctx, info); err != nil {
		t.Fatal(err)
	}
	if v, err := repo.Validators(ctx, "s"); err != nil || !v.IsZero() {
		t.Fatalf("fresh source validators %+v err %v", v, err)
	}
	if v, err := repo.Validators(ctx, "unknown"); err != nil || !v.IsZero() {
		t.Fatalf("unknown source validators %+v err %v", v, err)
	}

	for _, id := range []string{"s", "unknown"} {
		if last, err := repo.LastSuccess(ctx, id); err != nil || last != nil {
			t.Fatalf("%s: last success %v err %v before any poll", id, last, err)
		}
	}

	at := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	if err := repo.RecordSuccess(ctx, "s", at, domain.Validators{ETag: `"v1"`}); err != nil {
		t.Fatal(err)
	}
	if last, err := repo.LastSuccess(ctx, "s"); err != nil || last == nil || !last.Equal(at) {
		t.Fatalf("last success %v err %v, want %s", last, err, at)
	}
	if err := repo.RecordError(ctx, "s", at.Add(time.Minute), "HTTP 502"); err != nil {
		t.Fatal(err)
	}
	if v, _ := repo.Validators(ctx, "s"); v.ETag != `"v1"` || v.LastModified != "" {
		t.Fatalf("validators %+v", v)
	}

	// Re-registering refreshes metadata and keeps state.
	info.Name = "Renamed"
	if err := repo.Register(ctx, info); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list %+v err %v", list, err)
	}
	got := list[0]
	if got.Name != "Renamed" || got.PollInterval != time.Minute || got.LastError != "HTTP 502" ||
		got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(at) {
		t.Fatalf("status %+v", got)
	}
}
