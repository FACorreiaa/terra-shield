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
