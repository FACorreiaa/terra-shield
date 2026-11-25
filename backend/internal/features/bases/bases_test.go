package bases

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
	rows []domain.MilitaryBase
	got  *domain.BBox
}

func (f *fakeRepo) InBox(_ context.Context, bb *domain.BBox) ([]domain.MilitaryBase, error) {
	f.got = bb
	return f.rows, nil
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

func TestList(t *testing.T) {
	at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{rows: []domain.MilitaryBase{
		{QID: "Q161348", Name: "Ramstein Air Base", Kind: domain.BaseAir, Class: "air base", Operator: "United States Air Force", ISO2: "DE", Lon: 7.6, Lat: 49.44, UpdatedAt: at},
	}}
	w := get(t, repo, "/v1/bases?bbox=5,45,10,50")
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("%d %s %s", w.Code, w.Header().Get("Cache-Control"), w.Body)
	}
	if repo.got == nil || repo.got.MinLon != 5 {
		t.Errorf("bbox %+v", repo.got)
	}
	var body struct {
		Bases []struct {
			ID, Name, Kind, Class, Operator, Country, URL string
		} `json:"bases"`
		UpdatedAt   time.Time `json:"updatedAt"`
		Attribution string    `json:"attribution"`
		License     string    `json:"license"`
		Note        string    `json:"note"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	b := body.Bases[0]
	if len(body.Bases) != 1 || b.ID != "Q161348" || b.Kind != "air" || b.Country != "DE" ||
		b.URL != "https://www.wikidata.org/wiki/Q161348" || !body.UpdatedAt.Equal(at) ||
		body.Attribution != "Wikidata" || body.License != "CC0 1.0" || body.Note == "" {
		t.Fatalf("%+v", body)
	}

	if w := get(t, &fakeRepo{}, "/v1/bases"); w.Code != http.StatusOK || repo.got == nil {
		t.Errorf("worldwide: %d", w.Code)
	}
	if w := get(t, nil, "/v1/bases"); w.Code != http.StatusServiceUnavailable {
		t.Errorf("unconfigured: %d", w.Code)
	}
	if w := get(t, &fakeRepo{}, "/v1/bases?bbox=nope"); w.Code != http.StatusBadRequest {
		t.Errorf("bad bbox: %d", w.Code)
	}
}
