package aircraft

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shield/internal/aircraft"
	"shield/internal/config"
	"shield/internal/domain"
	"shield/internal/server"
)

type fakeFinder struct {
	res aircraft.Result
	err error
	got *domain.BBox
}

func (f *fakeFinder) InBox(_ context.Context, bb domain.BBox) (aircraft.Result, error) {
	f.got = &bb
	return f.res, f.err
}

func (*fakeFinder) MaxCells() int { return 6 }

func get(t *testing.T, f Finder, target string) *httptest.ResponseRecorder {
	t.Helper()
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	srv, err := server.New(d, []server.Feature{New(f)})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	return w
}

func TestList(t *testing.T) {
	at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	alt := 25975
	f := &fakeFinder{res: aircraft.Result{
		Aircraft: []domain.Aircraft{
			{Hex: "49530a", Callsign: "TAP215", Lon: -9.1, Lat: 38.7, AltitudeFt: &alt, PositionAt: at},
			{Hex: "ae57d4", Lon: -9.2, Lat: 38.8, OnGround: true, Military: true, PositionAt: at},
		},
		DataAt: at, Complete: true,
	}}
	w := get(t, f, "/v1/aircraft?bbox=-9.5,38.5,-9.0,39")
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "public, max-age=15" {
		t.Fatalf("%d %s %s", w.Code, w.Header().Get("Cache-Control"), w.Body)
	}
	if *f.got != (domain.BBox{MinLon: -9.5, MinLat: 38.5, MaxLon: -9, MaxLat: 39}) {
		t.Errorf("bbox %+v", *f.got)
	}
	var body struct {
		Aircraft    []map[string]any `json:"aircraft"`
		DataAt      *time.Time       `json:"dataAt"`
		Complete    bool             `json:"complete"`
		Attribution string           `json:"attribution"`
		License     string           `json:"license"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Aircraft) != 2 || !body.Complete || body.DataAt == nil || !body.DataAt.Equal(at) ||
		body.Attribution != "© adsb.lol contributors" || body.License != "ODbL 1.0" {
		t.Fatalf("%+v", body)
	}
	tap, mil := body.Aircraft[0], body.Aircraft[1]
	if tap["callsign"] != "TAP215" || tap["altitudeFt"] != float64(25975) || tap["military"] != false {
		t.Errorf("airborne %v", tap)
	}
	if _, ok := mil["altitudeFt"]; ok || mil["onGround"] != true || mil["military"] != true {
		t.Errorf("military on ground %v", mil)
	}
}

func TestListEmptyAndErrors(t *testing.T) {
	w := get(t, &fakeFinder{}, "/v1/aircraft?bbox=-9.5,38.5,-9.0,39")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"aircraft":[]`) ||
		strings.Contains(w.Body.String(), "dataAt") || !strings.Contains(w.Body.String(), `"complete":false`) {
		t.Errorf("nothing served: %s", w.Body)
	}
	if w := get(t, &fakeFinder{}, "/v1/aircraft"); w.Code != http.StatusBadRequest {
		t.Errorf("no bbox: %d", w.Code)
	}
	w = get(t, &fakeFinder{err: aircraft.ErrTooLarge}, "/v1/aircraft?bbox=-10,35,30,60")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "zoom in") {
		t.Errorf("too large: %d %s", w.Code, w.Body)
	}
	if w := get(t, nil, "/v1/aircraft?bbox=-9.5,38.5,-9.0,39"); w.Code != http.StatusServiceUnavailable {
		t.Errorf("unconfigured: %d", w.Code)
	}
}
