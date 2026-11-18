package fires

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

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

type fakeRepo struct {
	cells     []domain.FireCell
	pts       []domain.FireDetection
	truncated bool
	err       error

	gotQuery domain.FireQuery
	gotCell  float64
	gotLimit int
	called   string
}

func (f *fakeRepo) Grid(_ context.Context, q domain.FireQuery, cell float64) ([]domain.FireCell, error) {
	f.called, f.gotQuery, f.gotCell = "grid", q, cell
	return f.cells, f.err
}

func (f *fakeRepo) Points(_ context.Context, q domain.FireQuery, limit int) ([]domain.FireDetection, bool, error) {
	f.called, f.gotQuery, f.gotLimit = "points", q, limit
	return f.pts, f.truncated, f.err
}

func get(t *testing.T, r Repository, target string) *httptest.ResponseRecorder {
	t.Helper()
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	f := New(r)
	f.now = func() time.Time { return now }
	srv, err := server.New(d, []server.Feature{f})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	return w
}

type body struct {
	Type        string  `json:"type"`
	Aggregation string  `json:"aggregation"`
	CellDegrees float64 `json:"cellDegrees"`
	Truncated   bool    `json:"truncated"`
	Features    []struct {
		Type     string `json:"type"`
		Geometry struct {
			Type        string     `json:"type"`
			Coordinates [2]float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties map[string]any `json:"properties"`
	} `json:"features"`
}

func decode(t *testing.T, w *httptest.ResponseRecorder) body {
	t.Helper()
	var b body
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("%v: %s", err, w.Body)
	}
	return b
}

func TestGrid(t *testing.T) {
	frp := float32(9.5)
	r := &fakeRepo{cells: []domain.FireCell{{Lat: 45.5, Lon: 10.5, Count: 3, MaxFRP: &frp, LatestAt: now}}}

	w := get(t, r, "/v1/fires?zoom=2&bbox=-20,30,40,60")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/geo+json" ||
		w.Header().Get("Cache-Control") != "public, max-age=60" || w.Header().Get("ETag") == "" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	if r.called != "grid" || r.gotCell != CellDegrees(2) || r.gotQuery.BBox == nil || !r.gotQuery.Since.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("grid call %+v cell %v", r.gotQuery, r.gotCell)
	}
	b := decode(t, w)
	if b.Type != "FeatureCollection" || b.Aggregation != "grid" || b.CellDegrees != 2.8125 || len(b.Features) != 1 || b.Truncated {
		t.Fatalf("%+v", b)
	}
	f := b.Features[0]
	if f.Geometry.Type != "Point" || f.Geometry.Coordinates != [2]float64{10.5, 45.5} ||
		f.Properties["count"] != 3.0 || f.Properties["maxFrp"] != 9.5 || f.Properties["latestAt"] != "2026-11-16T12:00:00Z" {
		t.Fatalf("cell %+v", f)
	}

	// No zoom: the world grid; since is clamped to the 48 h window.
	if w := get(t, r, "/v1/fires?since=2026-11-01T00:00:00Z"); w.Code != http.StatusOK || r.gotCell != CellDegrees(0) ||
		!r.gotQuery.Since.Equal(now.Add(-48*time.Hour)) {
		t.Fatalf("%d cell %v since %s", w.Code, r.gotCell, r.gotQuery.Since)
	}
	recent := now.Add(-time.Hour)
	if get(t, r, "/v1/fires?since="+recent.Format(time.RFC3339)); !r.gotQuery.Since.Equal(recent) {
		t.Fatalf("since %s", r.gotQuery.Since)
	}
}

func TestPoints(t *testing.T) {
	frp := float32(4.2)
	r := &fakeRepo{
		pts: []domain.FireDetection{
			{Satellite: "NOAA-20", AcquiredAt: now, Lat: 39.123456789, Lon: -8.987654321, FRP: &frp, Confidence: "high", ISO2: "PT"},
			{Satellite: "NOAA-21", AcquiredAt: now, Lat: 30, Lon: -30},
		},
		truncated: true,
	}
	w := get(t, r, "/v1/fires?zoom=8&bbox=-10,38,-7,40")
	if w.Code != http.StatusOK || r.called != "points" || r.gotLimit != maxPoints {
		t.Fatalf("%d %s limit %d", w.Code, r.called, r.gotLimit)
	}
	b := decode(t, w)
	if b.Aggregation != "none" || !b.Truncated || len(b.Features) != 2 || b.CellDegrees != 0 {
		t.Fatalf("%+v", b)
	}
	p := b.Features[0]
	if p.Geometry.Coordinates != [2]float64{-8.98765, 39.12346} || p.Properties["satellite"] != "NOAA-20" ||
		p.Properties["frp"] != 4.2 || p.Properties["confidence"] != "high" || p.Properties["iso2"] != "PT" {
		t.Fatalf("point %+v", p)
	}
	for _, k := range []string{"frp", "confidence", "iso2"} {
		if _, ok := b.Features[1].Properties[k]; ok {
			t.Errorf("empty %s should be omitted: %v", k, b.Features[1].Properties)
		}
	}
}

func TestErrors(t *testing.T) {
	for target, want := range map[string]int{
		"/v1/fires?zoom=8":           http.StatusBadRequest, // points need a bbox
		"/v1/fires?zoom=30":          http.StatusBadRequest,
		"/v1/fires?bbox=1,2,3":       http.StatusBadRequest,
		"/v1/fires?since=last-night": http.StatusBadRequest,
	} {
		if w := get(t, &fakeRepo{}, target); w.Code != want {
			t.Errorf("%s: %d, want %d", target, w.Code, want)
		}
	}
	if w := get(t, &fakeRepo{err: errors.New("db down")}, "/v1/fires"); w.Code != http.StatusInternalServerError {
		t.Errorf("store error %d", w.Code)
	}
	if w := get(t, nil, "/v1/fires"); w.Code != http.StatusServiceUnavailable {
		t.Errorf("no database %d", w.Code)
	}
}

func TestCellDegrees(t *testing.T) {
	for z, want := range map[int]float64{0: 11.25, 2: 2.8125, 5: 0.3515625} {
		if got := CellDegrees(z); got != want {
			t.Errorf("zoom %d: %v, want %v", z, got, want)
		}
	}
}
