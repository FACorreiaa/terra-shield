package realtime

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"shield/internal/domain"
)

// fakeSource is an in-memory change log.
type fakeSource struct {
	mu      sync.Mutex
	log     []domain.StreamChange
	wake    func()
	failing bool
	ready   chan struct{}
}

func newFakeSource(initial ...domain.StreamChange) *fakeSource {
	return &fakeSource{log: initial, ready: make(chan struct{})}
}

func (f *fakeSource) Changes(_ context.Context, after int64, limit int) ([]domain.StreamChange, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing {
		return nil, false, errors.New("db down")
	}
	var out []domain.StreamChange
	for _, c := range f.log {
		if c.Seq > after {
			out = append(out, c)
		}
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

func (f *fakeSource) LatestChange(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.log) == 0 {
		return 0, nil
	}
	return f.log[len(f.log)-1].Seq, nil
}

func (f *fakeSource) Listen(ctx context.Context, _ *slog.Logger, wake func()) error {
	f.mu.Lock()
	f.wake = wake
	f.mu.Unlock()
	close(f.ready)
	<-ctx.Done()
	return nil
}

// append adds changes and notifies, like a committed write.
func (f *fakeSource) append(cs ...domain.StreamChange) {
	f.mu.Lock()
	f.log = append(f.log, cs...)
	wake := f.wake
	f.mu.Unlock()
	wake()
}

func quake(seq int64, id string, lon, lat float64, sev string) domain.StreamChange {
	return domain.StreamChange{
		Seq: seq, Op: domain.OpUpdated, EventID: id,
		Event: &domain.EventRecord{
			ID:        id,
			Event:     domain.Event{Type: domain.TypeEarthquake, Severity: sev},
			Countries: []domain.EventCountry{{ISO2: "PT"}},
		},
		Box: &domain.BBox{MinLon: lon, MinLat: lat, MaxLon: lon, MaxLat: lat},
	}
}

func start(t *testing.T, src *fakeSource, opts Options) (*Hub, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h, err := Start(ctx, src, slog.New(slog.DiscardHandler), opts)
	if err != nil {
		t.Fatal(err)
	}
	<-src.ready
	return h, cancel
}

func recv(t *testing.T, s *Subscription) domain.StreamChange {
	t.Helper()
	select {
	case c, ok := <-s.C:
		if !ok {
			t.Fatal("subscription closed")
		}
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("no change delivered")
	}
	return domain.StreamChange{}
}

func TestHubFansOutMatchingChanges(t *testing.T) {
	src := newFakeSource(quake(1, "old", 0, 0, "minor"))
	h, _ := start(t, src, Options{})

	all, _ := h.Subscribe(Filter{})
	iberia, _ := h.Subscribe(Filter{BBox: &domain.BBox{MinLon: -10, MinLat: 35, MaxLon: 5, MaxLat: 45}})
	severe, _ := h.Subscribe(Filter{Severities: []string{"severe"}, Country: "PT"})
	if all.Start != 1 {
		t.Fatalf("start %d, want the latest cursor 1", all.Start)
	}

	src.append(quake(2, "lisbon", -9, 38.7, "severe"), quake(3, "tokyo", 139, 35, "minor"),
		domain.StreamChange{Seq: 4, Op: domain.OpRemoved, EventID: "gone"})

	for _, want := range []int64{2, 3, 4} {
		if c := recv(t, all); c.Seq != want {
			t.Fatalf("all got %d, want %d", c.Seq, want)
		}
	}
	if c := recv(t, iberia); c.EventID != "lisbon" {
		t.Fatalf("iberia got %s", c.EventID)
	}
	if c := recv(t, iberia); c.Op != domain.OpRemoved {
		t.Fatalf("removals always pass, got %+v", c)
	}
	if c := recv(t, severe); c.EventID != "lisbon" {
		t.Fatalf("severe got %s", c.EventID)
	}
	if cur, idle := all.Idle(); !idle || cur != 4 {
		t.Fatalf("idle %v at %d", idle, cur)
	}
}

func TestHubDropsSlowSubscriberAndCaps(t *testing.T) {
	src := newFakeSource()
	h, cancel := start(t, src, Options{Buffer: 1, MaxSubscribers: 2})

	slow, _ := h.Subscribe(Filter{})
	fast, _ := h.Subscribe(Filter{})
	if _, err := h.Subscribe(Filter{}); !errors.Is(err, ErrFull) {
		t.Fatalf("third subscriber: %v", err)
	}
	src.append(quake(1, "a", 0, 0, "minor"))
	recv(t, fast)
	if _, idle := slow.Idle(); idle {
		t.Fatal("slow subscriber's queue should hold a change")
	}
	src.append(quake(2, "b", 0, 0, "minor"))
	recv(t, fast)
	<-slow.C // the queued change
	if _, ok := <-slow.C; ok {
		t.Fatal("slow subscriber not dropped")
	}
	slow.Close() // idempotent after a drop

	cancel()
	if _, ok := <-fast.C; ok {
		t.Fatal("shutdown left a subscription open")
	}
	if _, err := h.Subscribe(Filter{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("after shutdown: %v", err)
	}
}

func TestHubCatchUpAndRetry(t *testing.T) {
	src := newFakeSource(quake(1, "a", 0, 0, "minor"), quake(2, "b", 0, 0, "minor"), quake(3, "c", 0, 0, "minor"))
	h, _ := start(t, src, Options{PageSize: 2})

	got, complete, err := h.CatchUp(context.Background(), 0, 2, 10)
	if err != nil || !complete || len(got) != 2 {
		t.Fatalf("up to 2: %d complete=%v %v", len(got), complete, err)
	}
	if _, complete, _ := h.CatchUp(context.Background(), 0, 3, 2); complete {
		t.Fatal("3 changes in a budget of 2 should be incomplete")
	}
	if got, complete, _ := h.CatchUp(context.Background(), 0, 2, 2); !complete || len(got) != 2 {
		t.Fatal("exactly the budget, ending at upTo, is complete")
	}

	// A failed read keeps the cursor; the next wake publishes everything.
	sub, _ := h.Subscribe(Filter{})
	src.mu.Lock()
	src.failing = true
	src.mu.Unlock()
	src.append(quake(4, "d", 0, 0, "minor"))
	time.Sleep(50 * time.Millisecond)
	src.mu.Lock()
	src.failing = false
	src.mu.Unlock()
	src.append(quake(5, "e", 0, 0, "minor"), quake(6, "f", 0, 0, "minor"), quake(7, "g", 0, 0, "minor"))
	for _, want := range []int64{4, 5, 6, 7} { // paged by 2
		if c := recv(t, sub); c.Seq != want {
			t.Fatalf("got %d, want %d", c.Seq, want)
		}
	}
}
