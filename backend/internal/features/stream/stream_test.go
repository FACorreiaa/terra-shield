package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"shield/internal/config"
	"shield/internal/domain"
	"shield/internal/realtime"
	"shield/internal/server"
)

type fakeLog struct {
	mu   sync.Mutex
	log  []domain.StreamChange
	wake func()
	up   chan struct{}
}

func (f *fakeLog) Changes(_ context.Context, after int64, limit int) ([]domain.StreamChange, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

func (f *fakeLog) LatestChange(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.log) == 0 {
		return 0, nil
	}
	return f.log[len(f.log)-1].Seq, nil
}

func (f *fakeLog) Listen(ctx context.Context, _ *slog.Logger, wake func()) error {
	f.mu.Lock()
	f.wake = wake
	f.mu.Unlock()
	close(f.up)
	<-ctx.Done()
	return nil
}

func (f *fakeLog) add(cs ...domain.StreamChange) {
	f.mu.Lock()
	f.log = append(f.log, cs...)
	w := f.wake
	f.mu.Unlock()
	w()
}

var now = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

func change(seq int64, id string, typ domain.EventType, lon, lat float64) domain.StreamChange {
	return domain.StreamChange{
		Seq: seq, Op: domain.OpCreated, EventID: id,
		Event: &domain.EventRecord{
			Event: domain.Event{
				SourceEventID: id, Type: typ, Kind: domain.KindObserved, Status: domain.StatusActive,
				Severity: "moderate", Title: id, ObservedAt: now, UpdatedAt: now, Attrs: map[string]any{},
			},
			ID: id, SourceID: "usgs", FetchedAt: now, Latitude: lat, Longitude: lon,
		},
		Box: &domain.BBox{MinLon: lon, MinLat: lat, MaxLon: lon, MaxLat: lat},
	}
}

func serve(t *testing.T, src *fakeLog, beat time.Duration) *httptest.Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	hub, err := realtime.Start(ctx, src, slog.New(slog.DiscardHandler), realtime.Options{})
	if err != nil {
		t.Fatal(err)
	}
	<-src.up
	f := New(hub, nil)
	f.heartbeat = beat
	srv, err := server.New(&server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}, []server.Feature{f})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(func() { cancel(); ts.Close() })
	return ts
}

type msg struct{ id, event, data, retry string }

// open connects and returns a reader of SSE messages; id-only lines come
// back with an empty event.
func open(t *testing.T, ts *httptest.Server, query, lastID string) (int, http.Header, func() msg) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/stream"+query, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := ts.Client().Transport.RoundTrip(req) //nolint:bodyclose // closed in t.Cleanup; RoundTrip skips transparent gzip
	if err != nil {
		t.Fatal(err)
	}
	body := resp.Body
	t.Cleanup(func() { body.Close() })
	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	next := func() msg {
		t.Helper()
		var m msg
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					t.Fatal("stream ended")
				}
				switch {
				case l == "":
					if m != (msg{}) {
						return m
					}
				case strings.HasPrefix(l, "id: "):
					m.id = l[4:]
				case strings.HasPrefix(l, "event: "):
					m.event = l[7:]
				case strings.HasPrefix(l, "data: "):
					m.data = l[6:]
				case strings.HasPrefix(l, "retry: "):
					m.retry = l[7:]
				}
			case <-time.After(3 * time.Second):
				t.Fatal("no message")
			}
		}
	}
	return resp.StatusCode, resp.Header, next
}

func TestStreamLive(t *testing.T) {
	src := &fakeLog{up: make(chan struct{}), log: []domain.StreamChange{change(7, "old", domain.TypeEarthquake, 0, 0)}}
	ts := serve(t, src, time.Hour)
	status, h, next := open(t, ts, "?type=earthquake&bbox=-10,35,5,45", "")

	if ct := h.Get("Content-Type"); status != 200 || ct != "text/event-stream" || h.Get("Content-Encoding") != "" {
		t.Fatalf("%d %s enc=%q", status, ct, h.Get("Content-Encoding"))
	}
	if m := next(); m.retry != "5000" || m.event != "" {
		t.Fatalf("first %+v", m)
	}
	if m := next(); m.event != "ready" || m.id != "7" {
		t.Fatalf("ready %+v", m)
	}

	flood := change(9, "flood-pt", domain.TypeFlood, -8, 39)
	flood.Op = domain.OpUpdated
	src.add(flood, change(10, "quake-jp", domain.TypeEarthquake, 139, 35), change(11, "quake-pt", domain.TypeEarthquake, -8, 39),
		domain.StreamChange{Seq: 12, Op: domain.OpRemoved, EventID: "gone"})

	m := next()
	if m.event != "event.created" || m.id != "11" {
		t.Fatalf("filtered live message %+v", m)
	}
	var ev map[string]any
	if err := json.Unmarshal([]byte(m.data), &ev); err != nil || ev["id"] != "quake-pt" || ev["type"] != "earthquake" || ev["source"] != "usgs" {
		t.Fatalf("data %s %v", m.data, err)
	}
	if m := next(); m.event != "event.removed" || m.id != "12" || m.data != `{"id":"gone"}` {
		t.Fatalf("removal %+v", m)
	}
}

func TestStreamResume(t *testing.T) {
	var log []domain.StreamChange
	for i := int64(1); i <= 5; i++ {
		log = append(log, change(i, "e"+string(rune('0'+i)), domain.TypeEarthquake, 0, 0))
	}
	src := &fakeLog{up: make(chan struct{}), log: log}
	ts := serve(t, src, time.Hour)

	_, _, next := open(t, ts, "", "3")
	next() // retry
	if a, b, r := next(), next(), next(); a.id != "4" || b.id != "5" || r.event != "ready" || r.id != "5" {
		t.Fatalf("catch-up %+v %+v %+v", a, b, r)
	}

	_, _, next = open(t, ts, "", "99") // from the future: database reset
	next()
	if m := next(); m.event != "reset" || m.id != "5" {
		t.Fatalf("future cursor %+v", m)
	}
	if m := next(); m.event != "ready" {
		t.Fatalf("after reset %+v", m)
	}

	// Further behind than maxCatchUp: reset.
	var big []domain.StreamChange
	for i := int64(1); i <= maxCatchUp+5; i++ {
		big = append(big, change(i, "x", domain.TypeEarthquake, 0, 0))
	}
	ts = serve(t, &fakeLog{up: make(chan struct{}), log: big}, time.Hour)
	_, _, next = open(t, ts, "?lastEventId=1", "")
	next()
	if m := next(); m.event != "reset" {
		t.Fatalf("too far behind %+v", m)
	}
}

func TestStreamHeartbeatAdvancesCursor(t *testing.T) {
	src := &fakeLog{up: make(chan struct{}), log: []domain.StreamChange{change(1, "a", domain.TypeEarthquake, 0, 0)}}
	ts := serve(t, src, 50*time.Millisecond)
	_, _, next := open(t, ts, "?type=flood", "")
	next()
	next()                                               // ready at 1
	src.add(change(2, "b", domain.TypeEarthquake, 0, 0)) // filtered out
	for {
		m := next()
		if m.event != "" {
			t.Fatalf("unexpected %+v", m)
		}
		if m.id == "2" {
			return // heartbeat moved past the skipped change
		}
	}
}

func TestStreamErrors(t *testing.T) {
	ts := serve(t, &fakeLog{up: make(chan struct{})}, time.Hour)
	for _, q := range []string{"?type=meteor", "?bbox=1,2,3", "?country=PRT", "?severity=huge", "?lastEventId=abc", "?lastEventId=-1"} {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/v1/stream"+q, nil)
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d", q, resp.StatusCode)
		}
	}
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	srv, _ := server.New(d, []server.Feature{New(nil, nil)})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/stream", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("no broker: %d", w.Code)
	}
}
