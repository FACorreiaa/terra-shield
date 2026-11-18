package domain

import "testing"

func TestBBoxIntersects(t *testing.T) {
	box := func(a, b, c, d float64) *BBox { return &BBox{MinLon: a, MinLat: b, MaxLon: c, MaxLat: d} }
	cases := []struct {
		name string
		a, b *BBox
		want bool
	}{
		{"overlap", box(0, 0, 10, 10), box(5, 5, 15, 15), true},
		{"touching edge", box(0, 0, 10, 10), box(10, 0, 20, 10), true},
		{"apart in longitude", box(0, 0, 10, 10), box(11, 0, 20, 10), false},
		{"apart in latitude", box(0, 0, 10, 10), box(0, 11, 10, 20), false},
		{"viewport across antimeridian, east side", box(170, -20, -170, 0), box(178, -18, 179, -17), true},
		{"viewport across antimeridian, west side", box(170, -20, -170, 0), box(-179, -18, -178, -17), true},
		{"viewport across antimeridian, outside", box(170, -20, -170, 0), box(0, -18, 10, -17), false},
		{"both across", box(170, -20, -170, 0), box(175, -10, -175, 5), true},
	}
	for _, c := range cases {
		if got := c.a.Intersects(c.b); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
		if got := c.b.Intersects(c.a); got != c.want {
			t.Errorf("%s (swapped): %v", c.name, got)
		}
	}
}
