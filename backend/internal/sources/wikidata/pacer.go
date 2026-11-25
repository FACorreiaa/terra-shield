package wikidata

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"shield/internal/fetch"
)

// resumeFor is how long a poll keeps the classes it already fetched, so a
// River retry asks Wikidata only for the rest. A full poll is dozens of
// queries; a 429 or 502 on one of them (both seen on 2026-11-25) must not
// re-spend the whole budget.
const resumeFor = 6 * time.Hour

// pollAttempts: the first loads failed transiently (429, 502, a stream cut
// mid-body) up to twice in one poll, and River's backoff spreads 8
// attempts over about 80 minutes, inside resumeFor. A job that runs out
// is not retried until next week.
const pollAttempts = 8

// gate paces every query this process sends to the Wikidata Query Service.
// The service allows each client (IP and User-Agent) 60 s of query time a
// minute and answers 429 beyond it, and the sources that use it poll at the
// same moments (worker start), so the budget is shared: queries run one at
// a time, and each waits until the previous one's duration has passed
// since it ended, keeping the process at half the budget.
type gate struct {
	mu    sync.Mutex
	prev  time.Duration // how long the last query took
	ended time.Time     // when it ended
}

// service is the one gate for this process.
var service = &gate{}

// pacer runs one source's queries through the gate. It remembers each
// answer it parsed until the poll completes or resumeFor passes.
type pacer[T any] struct {
	endpoint string
	client   *fetch.Client
	gate     *gate
	now      func() time.Time
	pause    func(ctx context.Context, d time.Duration) error

	mu      sync.Mutex
	done    map[string]T
	started time.Time // when the answers in done began
}

func newPacer[T any](client *fetch.Client) *pacer[T] {
	return &pacer[T]{endpoint: Endpoint, client: client, gate: service, now: time.Now, pause: sleep}
}

// begin starts a poll: answers older than resumeFor are dropped.
func (p *pacer[T]) begin() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == nil || p.now().Sub(p.started) > resumeFor {
		started := p.now()
		p.done, p.started = map[string]T{}, started
		// Free the answers (tens of thousands of sites) if no poll
		// completes them, rather than holding them until next week.
		time.AfterFunc(resumeFor, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.started.Equal(started) {
				p.done = nil
			}
		})
	}
}

// finish ends a completed poll: the next one asks again.
func (p *pacer[T]) finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done = nil
}

// run returns key's answer from this poll, or fetches and parses query.
// label names the query in errors.
func (p *pacer[T]) run(ctx context.Context, key, label, query string, parse func([]byte) (T, error)) (T, error) {
	var zero T
	p.mu.Lock()
	held, ok := p.done[key]
	p.mu.Unlock()
	if ok {
		return held, nil
	}
	resp, err := p.fetch(ctx, query)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", label, err)
	}
	v, err := parse(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", label, err)
	}
	p.mu.Lock()
	if p.done != nil {
		p.done[key] = v
	}
	p.mu.Unlock()
	return v, nil
}

// fetch sends one query through the gate.
func (p *pacer[T]) fetch(ctx context.Context, query string) (*fetch.Response, error) {
	g := p.gate
	g.mu.Lock()
	defer g.mu.Unlock()
	if wait := g.prev - p.now().Sub(g.ended); wait > 0 && !g.ended.IsZero() {
		if err := p.pause(ctx, wait); err != nil {
			return nil, err
		}
	}
	start := p.now()
	resp, err := p.client.Get(ctx, p.endpoint+"?format=json&query="+url.QueryEscape(query), fetch.Validators{})
	g.ended = p.now()
	g.prev = g.ended.Sub(start)
	// After a 429, no source may ask again before the service says.
	var se *fetch.StatusError
	if errors.As(err, &se) && se.RetryAfter > g.prev {
		g.prev = se.RetryAfter
	}
	return resp, err
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
