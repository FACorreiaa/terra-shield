package sources

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shield/internal/config"
	"shield/internal/domain"
	"shield/internal/server"
)

func TestStale(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }

	cases := []struct {
		name    string
		enabled bool
		last    *time.Time
		want    bool
	}{
		{"never polled", true, nil, true},
		{"fresh", true, ago(2 * time.Minute), false},
		{"three intervals is not yet stale", true, ago(3 * time.Minute), false},
		{"past three intervals", true, ago(4 * time.Minute), true},
		{"disabled is never stale", false, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toWire([]domain.SourceStatus{{
				SourceInfo:    domain.SourceInfo{ID: "x", PollInterval: time.Minute, Enabled: c.enabled},
				LastSuccessAt: c.last,
			}}, now)
			if got[0].Stale != c.want {
				t.Fatalf("stale = %v, want %v", got[0].Stale, c.want)
			}
		})
	}
}

type fakeLister struct {
	rows []domain.SourceStatus
	err  error
}

func (f fakeLister) List(context.Context) ([]domain.SourceStatus, error) { return f.rows, f.err }

func serve(t *testing.T, l Lister) *httptest.ResponseRecorder {
	t.Helper()
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	srv, err := server.New(d, []server.Feature{New(l)})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/sources", nil))
	return rec
}

func TestListHandler(t *testing.T) {
	rec := serve(t, fakeLister{rows: []domain.SourceStatus{{
		SourceInfo: domain.SourceInfo{ID: "usgs", Name: "USGS", PollInterval: time.Minute, Enabled: true},
	}}})
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=30" {
		t.Fatalf("status %d headers %v", rec.Code, rec.Header())
	}
	var body listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Sources) != 1 ||
		body.Sources[0].PollIntervalSeconds != 60 || !body.Sources[0].Stale {
		t.Fatalf("body %s err %v", rec.Body, err)
	}

	if rec := serve(t, fakeLister{err: errors.New("db down")}); rec.Code != http.StatusInternalServerError {
		t.Fatalf("repository error → %d", rec.Code)
	}
	if rec := serve(t, nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no database → %d", rec.Code)
	}
}
