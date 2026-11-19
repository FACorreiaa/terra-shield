package params

import (
	"net/url"
	"testing"
	"time"
)

func TestBBox(t *testing.T) {
	bb, err := BBox(url.Values{"bbox": {" 170, -20 ,-170,-10"}})
	if err != nil || bb.MinLon != 170 || bb.MaxLon != -170 || bb.MinLat != -20 || bb.MaxLat != -10 {
		t.Fatalf("%+v %v", bb, err)
	}
	if bb, err := BBox(url.Values{}); bb != nil || err != nil {
		t.Fatalf("absent: %v %v", bb, err)
	}
	for _, s := range []string{"1,2,3", "a,0,1,1", "0,10,1,5", "-181,0,0,1", "0,0,181,1", "0,-91,1,0", "0,0,1,91"} {
		if _, err := BBox(url.Values{"bbox": {s}}); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestZoomAndTime(t *testing.T) {
	if z, err := Zoom(url.Values{"zoom": {"22"}}); err != nil || *z != 22 {
		t.Fatalf("%v %v", z, err)
	}
	for _, s := range []string{"-1", "23", "1.5"} {
		if _, err := Zoom(url.Values{"zoom": {s}}); err == nil {
			t.Errorf("zoom %q accepted", s)
		}
	}
	tm, err := Time(url.Values{"since": {"2026-11-16T01:00:00+01:00"}}, "since")
	if err != nil || !tm.Equal(time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)) || tm.Location() != time.UTC {
		t.Fatalf("%v %v", tm, err)
	}
	if _, err := Time(url.Values{"since": {"yesterday"}}, "since"); err == nil {
		t.Error("bad time accepted")
	}
}

func TestNear(t *testing.T) {
	n, err := Near(url.Values{"near": {" -9.14, 38.72"}})
	if err != nil || n.Lon != -9.14 || n.Lat != 38.72 || n.RadiusKm != DefaultRadiusKm {
		t.Fatalf("%+v %v", n, err)
	}
	if n, err := Near(url.Values{"near": {"0,0"}, "radiusKm": {"250.5"}}); err != nil || n.RadiusKm != 250.5 {
		t.Fatalf("radius: %+v %v", n, err)
	}
	if n, err := Near(url.Values{}); n != nil || err != nil {
		t.Fatalf("absent: %v %v", n, err)
	}
	for _, v := range []url.Values{
		{"near": {"1"}},
		{"near": {"1,2,3"}},
		{"near": {"a,0"}},
		{"near": {"181,0"}},
		{"near": {"0,-91"}},
		{"near": {"0,0"}, "radiusKm": {"0.5"}},
		{"near": {"0,0"}, "radiusKm": {"1001"}},
		{"near": {"0,0"}, "radiusKm": {"far"}},
		{"radiusKm": {"10"}},
	} {
		if _, err := Near(v); err == nil {
			t.Errorf("accepted %v", v)
		}
	}
}
