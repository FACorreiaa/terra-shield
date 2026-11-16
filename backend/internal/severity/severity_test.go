package severity

import "testing"

func TestEarthquake(t *testing.T) {
	cases := []struct {
		name    string
		mag     float64
		pager   string
		tsunami bool
		want    Level
	}{
		{"small", 2.8, "", false, Minor},
		{"just under moderate", 4.99, "", false, Minor},
		{"moderate band", 5.0, "", false, Moderate},
		{"severe band", 6.0, "", false, Severe},
		{"extreme band", 7.0, "", false, Extreme},
		{"PAGER green does not lower a big quake", 7.1, "green", false, Extreme},
		{"PAGER yellow raises a small one", 4.6, "yellow", false, Moderate},
		{"PAGER orange", 5.8, "orange", false, Severe},
		{"PAGER red", 6.2, "red", false, Extreme},
		{"unknown PAGER ignored", 4.0, "purple", false, Minor},
		{"tsunami flag floors at moderate", 4.8, "", true, Moderate},
		{"tsunami flag never lowers", 6.5, "", true, Severe},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Earthquake(c.mag, c.pager, c.tsunami); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestLevelOrderAndNames(t *testing.T) {
	if !(Minor < Moderate && Moderate < Severe && Severe < Extreme) {
		t.Fatal("levels out of order")
	}
	want := map[Level]string{Minor: "minor", Moderate: "moderate", Severe: "severe", Extreme: "extreme"}
	for l, s := range want {
		if l.String() != s {
			t.Errorf("%d = %q, want %q", l, l.String(), s)
		}
	}
	if Max() != Minor {
		t.Error("Max() of nothing should be Minor")
	}
}
