package historic

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shield/internal/config"
	"shield/internal/domain"
	"shield/internal/server"
)

type fakeRepo struct {
	got  domain.HistoricQuery
	cell float64
	pts  []domain.HistoricSite
	more bool
}

func (f *fakeRepo) Grid(_ context.Context, q domain.HistoricQuery, cell float64) ([]domain.HistoricCell, error) {
	f.got, f.cell = q, cell
	return []domain.HistoricCell{{Lat: 50, Lon: 10, Count: 3, Kinds: map[domain.HistoricKind]int{domain.HistoricCastle: 2, domain.HistoricMilitary: 1}}}, nil
}

func (f *fakeRepo) Points(_ context.Context, q domain.HistoricQuery, _ int) ([]domain.HistoricSite, bool, error) {
	f.got = q
	return f.pts, f.more, nil
}

func get(t *testing.T, r Repository, target string) *httptest.ResponseRecorder {
	t.Helper()
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	srv, err := server.New(d, []server.Feature{New(r)})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	return w
}

type body struct {
	Aggregation string  `json:"aggregation"`
	CellDegrees float64 `json:"cellDegrees"`
	Truncated   bool    `json:"truncated"`
	Attribution string  `json:"attribution"`
	Note        string  `json:"note"`
	UpdatedAt   *time.Time
	Features    []struct {
		Geometry struct {
			Coordinates [2]float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties map[string]any `json:"properties"`
	} `json:"features"`
}

func decode(t *testing.T, w *httptest.ResponseRecorder) body {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var b body
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGrid(t *testing.T) {
	repo := &fakeRepo{}
	w := get(t, repo, "/v1/historic?zoom=3&kind=castle,memorial")
	b := decode(t, w)
	if w.Header().Get("Content-Type") != mimeGeoJSON || w.Header().Get("Cache-Control") != "public, max-age=3600" {
		t.Errorf("headers %v", w.Header())
	}
	if b.Aggregation != "grid" || b.CellDegrees != CellDegrees(3) || repo.cell != CellDegrees(3) || b.Attribution != "Wikidata" || b.Note == "" {
		t.Fatalf("%+v", b)
	}
	if len(repo.got.Kinds) != 2 || repo.got.Kinds[0] != domain.HistoricCastle || repo.got.Kinds[1] != domain.HistoricMemorial {
		t.Errorf("kinds %v", repo.got.Kinds)
	}
	p := b.Features[0].Properties
	if p["count"] != 3.0 || p["kinds"].(map[string]any)["castle"] != 2.0 {
		t.Errorf("cell %v", p)
	}
	if w := get(t, repo, "/v1/historic"); w.Code != http.StatusOK || repo.cell != CellDegrees(0) || repo.got.Kinds != nil {
		t.Errorf("no zoom, no kind: %d cell %v kinds %v", w.Code, repo.cell, repo.got.Kinds)
	}
}

func TestPoints(t *testing.T) {
	at := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{more: true, pts: []domain.HistoricSite{
		{QID: "Q1", Name: "Castelo de São Jorge", Kind: domain.HistoricCastle, Class: "castle", ISO2: "PT", Lon: -9.133456, Lat: 38.713912, WorldHeritage: true, UpdatedAt: at},
	}}
	b := decode(t, get(t, repo, "/v1/historic?zoom=8&bbox=-10,38,-9,39&worldHeritage=true"))
	if b.Aggregation != "none" || !b.Truncated || b.UpdatedAt == nil || !b.UpdatedAt.Equal(at) || repo.got.BBox == nil || !repo.got.WorldHeritage {
		t.Fatalf("%+v", b)
	}
	f := b.Features[0]
	if f.Geometry.Coordinates != [2]float64{-9.13346, 38.71391} || f.Properties["id"] != "Q1" || f.Properties["kind"] != "castle" ||
		f.Properties["url"] != "https://www.wikidata.org/wiki/Q1" || f.Properties["country"] != "PT" || f.Properties["worldHeritage"] != true {
		t.Errorf("%+v", f)
	}
}

func TestErrors(t *testing.T) {
	for target, want := range map[string]int{
		"/v1/historic?zoom=8":              http.StatusBadRequest, // points need a bbox
		"/v1/historic?zoom=6":              http.StatusBadRequest, // so does a fine grid
		"/v1/historic?zoom=5":              http.StatusOK,
		"/v1/historic?zoom=7&bbox=0,0,1,1": http.StatusOK,
		"/v1/historic?kind=palace":         http.StatusBadRequest,
		"/v1/historic?kind=general":        http.StatusOK,
		"/v1/historic?worldHeritage=yes":   http.StatusBadRequest,
		"/v1/historic?bbox=1,2":            http.StatusBadRequest,
		"/v1/historic?zoom=8&bbox=0,0,1,1": http.StatusOK,
		// Points views are bounded: 30 by 20 degrees, across the
		// antimeridian too; the grid is not.
		"/v1/historic?zoom=8&bbox=-180,-90,180,90": http.StatusBadRequest,
		"/v1/historic?zoom=8&bbox=0,40,30,60":      http.StatusOK,
		"/v1/historic?zoom=8&bbox=0,40,31,60":      http.StatusBadRequest,
		"/v1/historic?zoom=8&bbox=0,40,30,61":      http.StatusBadRequest,
		"/v1/historic?zoom=9&bbox=170,0,-160,10":   http.StatusOK,
		"/v1/historic?zoom=9&bbox=170,0,-150,10":   http.StatusBadRequest,
		"/v1/historic?zoom=7&bbox=-180,-90,180,90": http.StatusOK,
	} {
		if w := get(t, &fakeRepo{}, target); w.Code != want {
			t.Errorf("%s: %d, want %d", target, w.Code, want)
		}
	}
	if w := get(t, nil, "/v1/historic"); w.Code != http.StatusServiceUnavailable {
		t.Errorf("unconfigured: %d", w.Code)
	}
}
