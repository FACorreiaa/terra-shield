package adsblol

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/point.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParse(t *testing.T) {
	area, err := Parse(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.UnixMilli(1795111276001).UTC(); !area.At.Equal(want) || area.QuotaRemaining != -1 {
		t.Fatalf("at %s remaining %d", area.At, area.QuotaRemaining)
	}
	m := map[string]domain.Aircraft{}
	for _, a := range area.Aircraft {
		m[a.Hex] = a
	}
	// 8 in the file: LADD (4b7f3f), no position (ae5f9e) and a position
	// older than 60 s (aaaaaa) are left out.
	if len(m) != 5 {
		t.Fatalf("got %d aircraft: %v", len(m), m)
	}
	for _, gone := range []string{"4b7f3f", "ae5f9e", "aaaaaa"} {
		if _, ok := m[gone]; ok {
			t.Errorf("%s should be left out", gone)
		}
	}
	tap := m["49530a"]
	if tap.Callsign != "TAP215" || tap.AltitudeFt == nil || *tap.AltitudeFt != 25975 || tap.OnGround ||
		tap.Military || tap.Emergency != "" || tap.GroundSpeedKt == nil || tap.TrackDeg == nil {
		t.Errorf("airborne: %+v", tap)
	}
	if g := m["490031"]; !g.OnGround || g.AltitudeFt != nil || g.Callsign != "" {
		t.Errorf("on ground: %+v", g)
	}
	if mil := m["ae57d4"]; !mil.Military || mil.Callsign != "C2010" {
		t.Errorf("military: %+v", mil)
	}
	if p := m["408305"]; !p.PositionAt.Equal(area.At.Add(-291 * time.Millisecond).Truncate(time.Second)) {
		t.Errorf("position at %s", p.PositionAt)
	}
}

func TestParseFailsClosed(t *testing.T) {
	for _, body := range []string{`{}`, `{"ac":[],"now":0}`, `{"now":1795111276001,"msg":"No error"}`, `[`} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s parsed", body)
		}
	}
	if a, err := Parse([]byte(`{"ac":[],"now":1795111276001}`)); err != nil || len(a.Aircraft) != 0 {
		t.Errorf("an empty sky is an answer: %v %v", a, err)
	}
	odd := `{"now":1795111276001,"ac":[
		{"hex":"aaaaa1","lat":38,"lon":-9,"seen_pos":1,"baro_rate":"fast"},
		{"hex":"aaaaa2","lat":38,"lon":-9,"seen_pos":1}]}`
	if a, err := Parse([]byte(odd)); err != nil || len(a.Aircraft) != 1 || a.Aircraft[0].Hex != "aaaaa2" {
		t.Errorf("one entry that does not decode must not cost the others: %+v %v", a, err)
	}
}

func TestArea(t *testing.T) {
	body := fixture(t)
	var path string
	limited := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if limited {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("X-Ratelimit-Remaining", "321")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	c := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), srv.URL+"/")

	area, err := c.Area(context.Background(), 38.5, -9, 120)
	if err != nil || area.QuotaRemaining != 321 || len(area.Aircraft) != 5 {
		t.Fatalf("%d aircraft, remaining %d, %v", len(area.Aircraft), area.QuotaRemaining, err)
	}
	if path != "/v2/point/38.5000/-9.0000/120" {
		t.Errorf("path %s", path)
	}
	if _, err := c.Area(context.Background(), 0, 0, MaxRadiusNM+1); err == nil {
		t.Error("radius above the API's cap accepted")
	}
	limited = true
	var qe *domain.QuotaError
	if _, err := c.Area(context.Background(), 38.5, -9, 120); !errors.As(err, &qe) || qe.RetryAfter != 2*time.Minute {
		t.Fatalf("429: %v", err)
	}
}

func TestReplay(t *testing.T) {
	now := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(Replay(func() time.Time { return now }))
	defer srv.Close()
	c := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), srv.URL)
	area, err := c.Area(context.Background(), 51.5, -0.1, 120) // London
	if err != nil || !area.At.Equal(now) || len(area.Aircraft) != 5 || area.QuotaRemaining != 500 {
		t.Fatalf("%+v %v", area, err)
	}
	for _, a := range area.Aircraft {
		if a.Lat < 49 || a.Lat > 54 || a.Lon < -4 || a.Lon > 4 {
			t.Errorf("%s at %.2f,%.2f is not around London", a.Hex, a.Lat, a.Lon)
		}
	}
}
