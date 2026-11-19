package domain

// Category groups event types into the map's menus (Weather, Quakes,
// Conflicts, …). The grouping lives here only; the API sends each event's
// category so clients never keep their own copy.
type Category string

// Categories, in menu order.
const (
	CategoryQuakes    Category = "quakes"
	CategoryWeather   Category = "weather"
	CategoryFloods    Category = "floods"
	CategoryFires     Category = "fires"
	CategoryVolcanoes Category = "volcanoes"
	CategoryConflicts Category = "conflicts"
	CategoryOther     Category = "other"
)

// categoryTypes is the grouping, every type exactly once.
var categoryTypes = []struct {
	category Category
	types    []EventType
}{
	{CategoryQuakes, []EventType{TypeEarthquake, TypeTsunami}},
	{CategoryWeather, []EventType{TypeStorm, TypeTropicalCyclone, TypeHeavyRain, TypeWind, TypeSnowIce, TypeHeat, TypeCold, TypeDrought}},
	{CategoryFloods, []EventType{TypeFlood}},
	{CategoryFires, []EventType{TypeWildfire}},
	{CategoryVolcanoes, []EventType{TypeVolcano}},
	{CategoryConflicts, []EventType{TypeConflict}},
	{CategoryOther, []EventType{TypeOther}},
}

// Categories lists every category in menu order.
func Categories() []Category {
	out := make([]Category, len(categoryTypes))
	for i, c := range categoryTypes {
		out[i] = c.category
	}
	return out
}

// Valid reports whether c is a known category.
func (c Category) Valid() bool {
	for _, ct := range categoryTypes {
		if ct.category == c {
			return true
		}
	}
	return false
}

// Types lists the event types in c.
func (c Category) Types() []EventType {
	for _, ct := range categoryTypes {
		if ct.category == c {
			return append([]EventType(nil), ct.types...)
		}
	}
	return nil
}

// Category is the menu t belongs to; an unknown type is "other".
func (t EventType) Category() Category {
	for _, ct := range categoryTypes {
		for _, x := range ct.types {
			if x == t {
				return ct.category
			}
		}
	}
	return CategoryOther
}

// Hazard reports whether t is a natural hazard. Conflicts are not: they
// appear only where a request names them (ADR 27).
func (t EventType) Hazard() bool { return t != TypeConflict }

// HazardTypes is every hazard type: what a request naming no type or
// category gets.
func HazardTypes() []EventType {
	var out []EventType
	for _, ct := range categoryTypes {
		for _, t := range ct.types {
			if t.Hazard() {
				out = append(out, t)
			}
		}
	}
	return out
}
