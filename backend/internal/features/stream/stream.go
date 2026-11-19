// Package stream serves GET /v1/stream (S2.6): live event changes as
// Server-Sent Events, filtered like /v1/events, resumable by Last-Event-ID.
//
// Protocol, per connection:
//  1. `retry: 5000`, the reconnect delay.
//  2. With a usable Last-Event-ID, the changes missed since it (catch-up);
//     with one too far behind (or ahead, after a database reset), `reset`,
//     and the client reloads /v1/events.
//  3. `ready`, whose id is the cursor the stream goes live from.
//  4. Live `event.created|updated|resolved` (data: one Event) and
//     `event.removed` (data: {"id"}) messages.
//
// Every 20 s an id-only line (no data, so no message is dispatched) is a
// heartbeat that also advances the client's Last-Event-ID past changes its
// filter skipped, keeping a resume short.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"shield/internal/domain"
	"shield/internal/features/events"
	"shield/internal/features/params"
	"shield/internal/httpx"
	"shield/internal/metrics"
	"shield/internal/realtime"
	"shield/internal/server"

	"github.com/labstack/echo/v5"
)

// Broker is the pod's change fan-out. realtime.Hub is one.
type Broker interface {
	Subscribe(f realtime.Filter, client string) (*realtime.Subscription, error)
	CatchUp(ctx context.Context, after, upTo int64, maxChanges int) ([]domain.StreamChange, bool, error)
}

const (
	// heartbeat is a contract (openapi.yaml, /v1/stream: "Every 20 s"):
	// the Apple client treats 45 s of silence as a dead connection, so
	// never raise it past 20 s.
	heartbeat = 20 * time.Second
	// maxCatchUp: a client further behind reloads rather than replays.
	maxCatchUp = 1000
	// writeTimeout drops a client whose connection stopped taking data.
	writeTimeout = 10 * time.Second
	retryMillis  = 5000
)

// Feature mounts /v1/stream.
type Feature struct {
	broker    Broker // nil when no database is configured (routes command)
	logger    *slog.Logger
	heartbeat time.Duration
}

// New builds the feature on a Broker.
func New(broker Broker, logger *slog.Logger) *Feature {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Feature{broker: broker, logger: logger, heartbeat: heartbeat}
}

// Name implements server.Feature.
func (*Feature) Name() string { return "stream" }

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/stream", f.stream) // = metrics.StreamRoute
	return nil
}

func parseFilter(c *echo.Context) (realtime.Filter, *int64, error) {
	var (
		fl  realtime.Filter
		err error
	)
	v := c.Request().URL.Query()
	if fl.BBox, err = params.BBox(v); err != nil {
		return fl, nil, err
	}
	if fl.Types, err = params.Types(v); err != nil {
		return fl, nil, err
	}
	if fl.Severities, err = params.Severities(v); err != nil {
		return fl, nil, err
	}
	if fl.Country, err = params.Country(v); err != nil {
		return fl, nil, err
	}
	// EventSource sends the header on reconnect; a first connection can
	// pass the cursor as ?lastEventId= instead.
	raw := c.Request().Header.Get("Last-Event-ID")
	if raw == "" {
		raw = v.Get("lastEventId")
	}
	if raw == "" {
		return fl, nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 0 {
		return fl, nil, httpx.BadRequest("Last-Event-ID must be a cursor from this stream")
	}
	return fl, &id, nil
}

func (f *Feature) stream(c *echo.Context) error {
	if f.broker == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	filter, lastID, err := parseFilter(c)
	if err != nil {
		return err
	}
	sub, err := f.broker.Subscribe(filter, c.RealIP())
	switch {
	case errors.Is(err, realtime.ErrClientLimit):
		c.Response().Header().Set("Retry-After", "30")
		return httpx.Abort(http.StatusTooManyRequests, "too many open streams from this address")
	case errors.Is(err, realtime.ErrFull) || errors.Is(err, realtime.ErrClosed):
		c.Response().Header().Set("Retry-After", "30")
		return httpx.Abort(http.StatusServiceUnavailable, "stream busy, retry later")
	}
	if err != nil {
		return err
	}
	defer sub.Close()

	h := c.Response().Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no") // no proxy buffering
	c.Response().WriteHeader(http.StatusOK)

	ctx := c.Request().Context()
	w := &writer{w: c.Response(), rc: http.NewResponseController(c.Response())}
	if err := f.open(ctx, w, sub, &filter, lastID); err != nil {
		f.logEnd(ctx, err)
		return nil // headers are sent; nothing left to report to the client
	}
	f.logEnd(ctx, f.live(ctx, w, sub, &filter))
	return nil
}

// open sends the reconnect delay, the catch-up or a reset, then `ready`.
func (f *Feature) open(ctx context.Context, w *writer, sub *realtime.Subscription, filter *realtime.Filter, lastID *int64) error {
	if err := w.send(fmt.Sprintf("retry: %d\n\n", retryMillis)); err != nil {
		return err
	}
	if lastID != nil {
		switch {
		case *lastID > sub.Start:
			// A cursor from the future: the database was reset.
			if err := w.message("reset", sub.Start, []byte("{}")); err != nil {
				return err
			}
		case *lastID < sub.Start:
			changes, complete, err := f.broker.CatchUp(ctx, *lastID, sub.Start, maxCatchUp)
			if err != nil {
				return err
			}
			if !complete {
				if err := w.message("reset", sub.Start, []byte("{}")); err != nil {
					return err
				}
				break
			}
			for i := range changes {
				if !filter.Match(&changes[i]) {
					continue
				}
				if err := w.change(&changes[i]); err != nil {
					return err
				}
			}
		}
	}
	return w.message("ready", sub.Start, []byte("{}"))
}

// live relays the subscription until the client leaves, the hub closes,
// or a write fails. When the queue overflowed (a burst), it replays the
// gap from the change log in place; only a gap too large to replay ends
// the connection, and the client's reconnect then gets `reset`.
func (f *Feature) live(ctx context.Context, w *writer, sub *realtime.Subscription, filter *realtime.Filter) error {
	tick := time.NewTicker(f.heartbeat)
	defer tick.Stop()
	cursor := sub.Start
	for {
		select {
		case <-ctx.Done():
			return nil
		case ch, ok := <-sub.C:
			if !ok {
				return errDropped
			}
			if ch.Seq <= cursor {
				continue
			}
			if err := w.change(&ch); err != nil {
				return err
			}
			cursor = ch.Seq
		case <-sub.Lag:
			upTo := sub.Resume()
			changes, complete, err := f.broker.CatchUp(ctx, cursor, upTo, maxCatchUp)
			if err != nil {
				return err
			}
			if !complete {
				metrics.StreamDropped.Inc()
				return errDropped
			}
			for i := range changes {
				if changes[i].Seq <= cursor || !filter.Match(&changes[i]) {
					continue
				}
				if err := w.change(&changes[i]); err != nil {
					return err
				}
			}
			cursor = upTo
		case <-tick.C:
			// Advance the client's cursor only when nothing is queued, so
			// it never skips past an unsent change.
			if seq, idle := sub.Idle(); idle && seq > cursor {
				cursor = seq
			}
			if err := w.send("id: " + strconv.FormatInt(cursor, 10) + "\n\n"); err != nil {
				return err
			}
		}
	}
}

var errDropped = errors.New("stream closed: shutdown, or too far behind to replay")

func (f *Feature) logEnd(ctx context.Context, err error) {
	if err != nil && ctx.Err() == nil {
		f.logger.Debug("stream ended", "error", err)
	}
}

// writer frames SSE messages and flushes each, with a write deadline.
type writer struct {
	w  io.Writer
	rc *http.ResponseController
}

func (w *writer) send(s string) error {
	// Not every ResponseWriter supports deadlines (tests); a missing one
	// only loses the slow-client guard.
	_ = w.rc.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := io.WriteString(w.w, s); err != nil {
		return err
	}
	return w.rc.Flush()
}

func (w *writer) message(event string, id int64, data []byte) error {
	return w.send("id: " + strconv.FormatInt(id, 10) + "\nevent: " + event + "\ndata: " + string(data) + "\n\n")
}

func (w *writer) change(c *domain.StreamChange) error {
	var (
		data []byte
		err  error
	)
	if c.Event == nil {
		data, err = json.Marshal(struct {
			ID string `json:"id"`
		}{c.EventID})
	} else {
		data, err = json.Marshal(events.FromRecord(c.Event))
	}
	if err != nil {
		return err
	}
	return w.message("event."+string(c.Op), c.Seq, data)
}
