// Package fetch is the HTTP client every feed adapter uses: conditional
// GET, a body cap, per-host politeness, and Retry-After surfaced to the
// caller. Adapted from the in-cluster feeds service (apps/feeds/internal/
// fetch and poll/hostlimit), minus its SSRF guard: Shield's feed URLs are
// constants in code, never user input.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"shield/internal/domain"
)

// ErrTooLarge is returned when a body exceeds Options.MaxBody.
var ErrTooLarge = errors.New("fetch: response body too large")

// Validators are the conditional-GET values from the previous good fetch.
type Validators = domain.Validators

// Response is a completed fetch. Body is nil when NotModified.
type Response struct {
	Body        []byte
	Validators  Validators
	NotModified bool
	// Header is the response's headers, for feeds that report more than
	// validators (adsb.lol's remaining quota).
	Header http.Header
}

// StatusError is a non-2xx, non-304 answer.
type StatusError struct {
	URL        string
	Status     int
	RetryAfter time.Duration // zero when the server sent none
}

func (e *StatusError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("GET %s: HTTP %d (retry after %s)", e.URL, e.Status, e.RetryAfter)
	}
	return fmt.Sprintf("GET %s: HTTP %d", e.URL, e.Status)
}

// Options tunes a Client.
type Options struct {
	Timeout   time.Duration // whole request, headers and body; default 30s
	MaxBody   int64         // decoded bytes; default 16 MiB
	UserAgent string
	HostGap   time.Duration // minimum gap between requests to one host; default 1s
}

// Client is safe for concurrent use.
type Client struct {
	http      *http.Client
	maxBody   int64
	userAgent string
	hosts     *hostLimiter
}

// New builds a Client.
func New(o Options) *Client {
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.MaxBody == 0 {
		o.MaxBody = 16 << 20
	}
	if o.HostGap == 0 {
		o.HostGap = time.Second
	}
	if o.UserAgent == "" {
		o.UserAgent = "Shield-HazardMap/0.1"
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = o.Timeout
	return &Client{
		// The transport negotiates gzip itself; MaxBody applies to decoded bytes.
		http:      &http.Client{Transport: transport, Timeout: o.Timeout},
		maxBody:   o.MaxBody,
		userAgent: o.UserAgent,
		hosts:     &hostLimiter{gap: o.HostGap, slots: map[string]*hostSlot{}},
	}
}

// Get performs a conditional GET. Some servers (USGS behind its CDN) ignore
// If-None-Match/If-Modified-Since and answer 200 with the same validator;
// Get reports that as NotModified too, so callers skip parsing and writes.
func (c *Client) Get(ctx context.Context, url string, v Validators) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if v.ETag != "" {
		req.Header.Set("If-None-Match", v.ETag)
	}
	if v.LastModified != "" {
		req.Header.Set("If-Modified-Since", v.LastModified)
	}

	release, err := c.hosts.acquire(ctx, req.URL.Host)
	if err != nil {
		return nil, err
	}
	defer release()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	got := Validators{ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}
	switch {
	case resp.StatusCode == http.StatusNotModified:
		return &Response{Validators: v, NotModified: true, Header: resp.Header}, nil
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, &StatusError{URL: url, Status: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}
	case sameVersion(v, got):
		return &Response{Validators: got, NotModified: true, Header: resp.Header}, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("GET %s: read body: %w", url, err)
	}
	if int64(len(body)) > c.maxBody {
		return nil, fmt.Errorf("GET %s: %w", url, ErrTooLarge)
	}
	return &Response{Body: body, Validators: got, Header: resp.Header}, nil
}

// Head fetches only the validators of url. For feeds that ignore
// conditional GET (FIRMS' static CSVs), a HEAD and SameVersion skip a
// multi-megabyte download when nothing changed.
func (c *Client) Head(ctx context.Context, url string) (Validators, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, http.NoBody)
	if err != nil {
		return Validators{}, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	release, err := c.hosts.acquire(ctx, req.URL.Host)
	if err != nil {
		return Validators{}, err
	}
	defer release()
	resp, err := c.http.Do(req)
	if err != nil {
		return Validators{}, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Validators{}, &StatusError{URL: url, Status: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}
	}
	return Validators{ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}, nil
}

// SameVersion reports whether got carries the validator already held in
// held. An ETag decides when both sides have one; otherwise Last-Modified.
func SameVersion(held, got Validators) bool {
	return sameVersion(held, got)
}

// sameVersion reports a 200 that carries the validator we already hold.
func sameVersion(sent, got Validators) bool {
	if sent.ETag != "" && got.ETag != "" {
		return sent.ETag == got.ETag
	}
	return sent.LastModified != "" && sent.LastModified == got.LastModified
}

// parseRetryAfter handles both delta-seconds and HTTP-date forms.
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// hostLimiter serializes requests per host with a minimum gap, so no source
// ever sees Shield as a burst.
type hostLimiter struct {
	gap   time.Duration
	mu    sync.Mutex
	slots map[string]*hostSlot
}

type hostSlot struct {
	sem  chan struct{}
	last time.Time
}

func (l *hostLimiter) acquire(ctx context.Context, host string) (func(), error) {
	l.mu.Lock()
	s, ok := l.slots[host]
	if !ok {
		s = &hostSlot{sem: make(chan struct{}, 1)}
		l.slots[host] = s
	}
	l.mu.Unlock()

	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if wait := l.gap - time.Since(s.last); wait > 0 {
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			<-s.sem
			return nil, ctx.Err()
		}
	}
	return func() {
		s.last = time.Now()
		<-s.sem
	}, nil
}
