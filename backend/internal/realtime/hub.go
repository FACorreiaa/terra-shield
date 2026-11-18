// Package realtime fans event changes out to live-stream subscribers
// (S2.6). One Hub per API pod: a single LISTEN connection wakes it, it reads
// the change log once by cursor, and each subscriber receives the changes
// its filter matches. Many pods work alike because the cursor lives in the
// database, not in the hub.
package realtime

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"shield/internal/domain"
	"shield/internal/metrics"
)

// Source reads the event change log. store.EventRepository is one.
type Source interface {
	// Changes returns up to limit changes after cursor `after` and
	// whether more follow.
	Changes(ctx context.Context, after int64, limit int) ([]domain.StreamChange, bool, error)
	// LatestChange is the highest committed cursor.
	LatestChange(ctx context.Context) (int64, error)
	// Listen calls wake on every change notification until ctx ends.
	Listen(ctx context.Context, logger *slog.Logger, wake func()) error
}

var (
	// ErrFull means the pod is at its subscriber limit.
	ErrFull = errors.New("realtime: subscriber limit reached")
	// ErrClosed means the hub is shutting down.
	ErrClosed = errors.New("realtime: hub closed")
)

// Options tune a Hub; zero values take the defaults.
type Options struct {
	MaxSubscribers int           // default 2000
	Buffer         int           // per-subscriber queue; default 256
	PageSize       int           // changes per read; default 500
	SafetyPoll     time.Duration // re-read even without a notification; default 30 s
}

func (o *Options) defaults() {
	if o.MaxSubscribers <= 0 {
		o.MaxSubscribers = 2000
	}
	if o.Buffer <= 0 {
		o.Buffer = 256
	}
	if o.PageSize <= 0 {
		o.PageSize = 500
	}
	if o.SafetyPoll <= 0 {
		o.SafetyPoll = 30 * time.Second
	}
}

// Hub is the pod's change fan-out.
type Hub struct {
	src    Source
	logger *slog.Logger
	opts   Options
	wakeCh chan struct{}

	mu     sync.Mutex
	seq    int64 // every change up to seq has been published
	subs   map[*Subscription]struct{}
	closed bool
}

// Start reads the current cursor, then runs the listener and the fan-out
// until ctx ends, when every subscription is closed.
func Start(ctx context.Context, src Source, logger *slog.Logger, opts Options) (*Hub, error) {
	opts.defaults()
	seq, err := src.LatestChange(ctx)
	if err != nil {
		return nil, err
	}
	h := &Hub{
		src: src, logger: logger, opts: opts, seq: seq,
		wakeCh: make(chan struct{}, 1), subs: map[*Subscription]struct{}{},
	}
	go func() {
		if err := src.Listen(ctx, logger, h.wake); err != nil {
			logger.Error("event listener stopped", "error", err)
		}
	}()
	go h.run(ctx)
	return h, nil
}

// wake asks for a read; wakes arriving during a read coalesce into one.
func (h *Hub) wake() {
	select {
	case h.wakeCh <- struct{}{}:
	default:
	}
}

func (h *Hub) run(ctx context.Context) {
	tick := time.NewTicker(h.opts.SafetyPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			h.close()
			return
		case <-h.wakeCh:
		case <-tick.C:
		}
		h.drain(ctx)
	}
}

// drain publishes everything after the cursor. A failed read is retried on
// the next wake or tick; the cursor only moves past published changes.
func (h *Hub) drain(ctx context.Context) {
	for {
		changes, more, err := h.src.Changes(ctx, h.cursor(), h.opts.PageSize)
		if err != nil {
			if ctx.Err() == nil {
				h.logger.Warn("reading event changes", "error", err)
			}
			return
		}
		for i := range changes {
			h.publish(&changes[i])
		}
		if !more {
			return
		}
	}
}

func (h *Hub) cursor() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seq
}

// publish advances the cursor and queues c for every matching subscriber.
// A subscriber whose queue is full is dropped rather than slowing the
// others; it reconnects with Last-Event-ID and catches up.
func (h *Hub) publish(c *domain.StreamChange) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq = c.Seq
	for s := range h.subs {
		if !s.filter.Match(c) {
			continue
		}
		select {
		case s.ch <- *c:
		default:
			h.removeLocked(s)
			metrics.StreamDropped.Inc()
		}
	}
}

func (h *Hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for s := range h.subs {
		h.removeLocked(s)
	}
}

func (h *Hub) removeLocked(s *Subscription) {
	if _, ok := h.subs[s]; !ok {
		return
	}
	delete(h.subs, s)
	close(s.ch)
	metrics.StreamSubscribers.Dec()
}

// Subscribe registers a subscriber. Its Start is the cursor at
// registration: every later matching change arrives on C; anything at or
// before Start must be read with CatchUp.
func (h *Hub) Subscribe(f Filter) (*Subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrClosed
	}
	if len(h.subs) >= h.opts.MaxSubscribers {
		return nil, ErrFull
	}
	ch := make(chan domain.StreamChange, h.opts.Buffer)
	s := &Subscription{C: ch, Start: h.seq, ch: ch, hub: h, filter: f}
	h.subs[s] = struct{}{}
	metrics.StreamSubscribers.Inc()
	return s, nil
}

// CatchUp returns the changes after cursor `after` and up to `upTo` (a
// subscription's Start), unfiltered. complete is false when more than max
// would be needed: the client should reload instead.
func (h *Hub) CatchUp(ctx context.Context, after, upTo int64, maxChanges int) ([]domain.StreamChange, bool, error) {
	// One past the budget tells "exactly maxChanges" from "more".
	changes, more, err := h.src.Changes(ctx, after, maxChanges+1)
	if err != nil {
		return nil, false, err
	}
	// Events changed again after upTo now sit past it and arrive live.
	n := len(changes)
	for n > 0 && changes[n-1].Seq > upTo {
		n--
	}
	if n > maxChanges {
		return nil, false, nil
	}
	return changes[:n], !more || n < len(changes), nil
}

// Subscription is one stream client.
type Subscription struct {
	// C carries matching changes in cursor order. It is closed when the
	// subscriber is dropped for falling behind or the hub shuts down.
	C <-chan domain.StreamChange
	// Start is the cursor at registration.
	Start int64

	ch     chan domain.StreamChange
	hub    *Hub
	filter Filter
}

// Close unregisters the subscription; safe to call more than once.
func (s *Subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	s.hub.removeLocked(s)
}

// Idle returns the hub cursor if every change up to it has been taken from
// C, so a client told this cursor misses nothing; ok is false while C still
// holds changes. Only C's reader may call it.
func (s *Subscription) Idle() (cursor int64, ok bool) {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	// publish sends under the same lock that advances seq, so an empty
	// queue here means everything up to seq was already read.
	return s.hub.seq, len(s.ch) == 0
}

// Filter selects changes; zero fields match everything. Removals always
// match: they carry no event to filter on, and dropping an unknown id is
// free for a client.
type Filter struct {
	BBox       *domain.BBox
	Types      []domain.EventType
	Severities []string
	Country    string
}

// Match reports whether c passes the filter.
func (f *Filter) Match(c *domain.StreamChange) bool {
	e := c.Event
	if e == nil {
		return true
	}
	if len(f.Types) > 0 && !slices.Contains(f.Types, e.Type) {
		return false
	}
	if len(f.Severities) > 0 && !slices.Contains(f.Severities, e.Severity) {
		return false
	}
	if f.Country != "" && !slices.ContainsFunc(e.Countries, func(ec domain.EventCountry) bool { return ec.ISO2 == f.Country }) {
		return false
	}
	if f.BBox != nil && c.Box != nil && !f.BBox.Intersects(c.Box) {
		return false
	}
	return true
}
