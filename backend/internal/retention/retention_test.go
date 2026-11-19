package retention

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"shield/internal/domain"
)

type fakeStore struct {
	purges   []int // what each PurgeResolved call returns
	calls    int
	before   time.Time
	pruned   time.Time
	purgeErr error
}

func (f *fakeStore) PurgeResolved(_ context.Context, before time.Time, limit int) (int, error) {
	f.before = before
	if limit != batch {
		return 0, errors.New("unexpected batch size")
	}
	n := 0
	if f.calls < len(f.purges) {
		n = f.purges[f.calls]
	}
	f.calls++
	return n, f.purgeErr
}

func (f *fakeStore) PruneTombstones(_ context.Context, before time.Time) (int, error) {
	f.pruned = before
	return 3, nil
}

func task(s Store, now time.Time) *Task {
	t := New(s, slog.New(slog.DiscardHandler))
	t.now = func() time.Time { return now }
	t.pause = 0
	return t
}

func TestRun(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	s := &fakeStore{purges: []int{batch, batch, 50}}
	if err := task(s, now).Run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if s.calls != 3 || !s.before.Equal(now.Add(-domain.ResolvedRetention)) || !s.pruned.Equal(now.Add(-TombstoneRetention)) {
		t.Errorf("calls %d before %v pruned %v", s.calls, s.before, s.pruned)
	}

	// A backlog stops at maxBatches; the next run carries on.
	full := make([]int, 100)
	for i := range full {
		full[i] = batch
	}
	s = &fakeStore{purges: full}
	if err := task(s, now).Run(t.Context()); err != nil || s.calls != maxBatches {
		t.Errorf("backlog: %d calls err %v, want %d", s.calls, err, maxBatches)
	}

	s = &fakeStore{purgeErr: errors.New("db down")}
	if err := task(s, now).Run(t.Context()); err == nil || !s.pruned.IsZero() {
		t.Errorf("purge error: err %v, pruned anyway %v", err, s.pruned)
	}
}
