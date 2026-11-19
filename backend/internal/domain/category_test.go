package domain

import (
	"slices"
	"testing"
)

func TestCategories(t *testing.T) {
	all := []EventType{
		TypeEarthquake, TypeWildfire, TypeFlood, TypeStorm, TypeTropicalCyclone, TypeVolcano, TypeDrought,
		TypeHeat, TypeCold, TypeHeavyRain, TypeWind, TypeSnowIce, TypeTsunami, TypeOther, TypeConflict,
	}
	seen := map[EventType]Category{}
	for _, c := range Categories() {
		if !c.Valid() {
			t.Errorf("%s not valid", c)
		}
		for _, typ := range c.Types() {
			if prev, dup := seen[typ]; dup {
				t.Errorf("%s in %s and %s", typ, prev, c)
			}
			seen[typ] = c
			if typ.Category() != c {
				t.Errorf("%s.Category() = %s, want %s", typ, typ.Category(), c)
			}
		}
	}
	for _, typ := range all {
		if !typ.Valid() {
			t.Errorf("%s not valid", typ)
		}
		if _, ok := seen[typ]; !ok {
			t.Errorf("%s in no category", typ)
		}
	}
	if len(seen) != len(all) {
		t.Errorf("categories hold %d types, want %d", len(seen), len(all))
	}
	if EventType("meteor").Category() != CategoryOther || Category("sport").Valid() {
		t.Error("unknown values")
	}
	h := HazardTypes()
	if slices.Contains(h, TypeConflict) || len(h) != len(all)-1 || TypeConflict.Hazard() || !TypeFlood.Hazard() {
		t.Errorf("hazards %v", h)
	}
	// Types returns a copy.
	q := CategoryQuakes.Types()
	q[0] = TypeOther
	if CategoryQuakes.Types()[0] != TypeEarthquake {
		t.Error("Types leaked its backing array")
	}
}
