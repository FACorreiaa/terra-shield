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

func TestGDACSAlert(t *testing.T) {
	cases := map[string]Level{"Green": Minor, "orange": Severe, "RED": Extreme}
	for in, want := range cases {
		if got, ok := GDACSAlert(in); !ok || got != want {
			t.Errorf("%q = %s/%v, want %s", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "Yellow"} {
		if _, ok := GDACSAlert(in); ok {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestCycloneWindKmh(t *testing.T) {
	cases := map[float64]Level{0: Minor, 118: Minor, 119: Moderate, 177: Moderate, 178: Severe, 251: Severe, 252: Extreme}
	for kmh, want := range cases {
		if got := CycloneWindKmh(kmh); got != want {
			t.Errorf("%v km/h = %s, want %s", kmh, got, want)
		}
	}
}

func TestWildfireAcres(t *testing.T) {
	cases := map[float64]Level{0: Minor, 999: Minor, 1000: Moderate, 9999: Moderate, 10000: Severe, 99999: Severe, 100000: Extreme}
	for acres, want := range cases {
		if got := WildfireAcres(acres); got != want {
			t.Errorf("%v acres = %s, want %s", acres, got, want)
		}
	}
}

func TestMeteoalarmAwareness(t *testing.T) {
	for in, want := range map[int]Level{2: Moderate, 3: Severe, 4: Extreme} {
		if got, ok := MeteoalarmAwareness(in); !ok || got != want {
			t.Errorf("%d = %s/%v, want %s", in, got, ok, want)
		}
	}
	for _, in := range []int{0, 1, 5} {
		if _, ok := MeteoalarmAwareness(in); ok {
			t.Errorf("%d accepted", in)
		}
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
	for l := Minor; l <= Extreme; l++ {
		if got, ok := Parse(l.String()); !ok || got != l {
			t.Errorf("Parse(%q) = %v, %v", l.String(), got, ok)
		}
	}
	if _, ok := Parse("Severe"); ok {
		t.Error("Parse is case-sensitive: wire values are lower case")
	}
	if Max() != Minor {
		t.Error("Max() of nothing should be Minor")
	}
}

func TestConflictFatalities(t *testing.T) {
	for best, want := range map[int]Level{0: Minor, 4: Minor, 5: Moderate, 24: Moderate, 25: Severe, 99: Severe, 100: Extreme, 5714: Extreme} {
		if got := ConflictFatalities(best); got != want {
			t.Errorf("%d deaths: %v, want %v", best, got, want)
		}
	}
}
