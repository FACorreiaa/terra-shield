package params

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"shield/internal/domain"
	"shield/internal/httpx"
	"shield/internal/severity"
)

// MaxListItems bounds a comma-separated parameter.
const MaxListItems = 20

var countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

// List reads a comma-separated parameter, dropping empty items and
// duplicates; parse validates each item. A bad item is a 400 naming it.
func List[T comparable](v url.Values, name string, parse func(string) (T, bool)) ([]T, error) {
	s := v.Get(name)
	if s == "" {
		return nil, nil
	}
	var out []T
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		val, ok := parse(item)
		if !ok {
			return nil, httpx.BadRequest(name + ": unknown value " + strconv.Quote(item))
		}
		if !slices.Contains(out, val) {
			out = append(out, val)
		}
	}
	if len(out) > MaxListItems {
		return nil, httpx.BadRequest(name + ": too many values")
	}
	return out, nil
}

// Types reads "type" (event types) and "category" (menus, each standing
// for its types) into the types to match. Neither gives every hazard type:
// conflicts appear only when named (ADR 27). Both match the types in
// both; none in common is a 400, never "anything".
func Types(v url.Values) ([]domain.EventType, error) {
	types, err := List(v, "type", func(s string) (domain.EventType, bool) {
		return domain.EventType(s), domain.EventType(s).Valid()
	})
	if err != nil {
		return nil, err
	}
	cats, err := List(v, "category", func(s string) (domain.Category, bool) {
		return domain.Category(s), domain.Category(s).Valid()
	})
	if err != nil {
		return nil, err
	}
	switch {
	case len(types) == 0 && len(cats) == 0:
		return domain.HazardTypes(), nil
	case len(cats) == 0:
		return types, nil
	}
	var inCats []domain.EventType
	for _, c := range cats {
		inCats = append(inCats, c.Types()...)
	}
	if len(types) == 0 {
		return inCats, nil
	}
	var both []domain.EventType
	for _, t := range types {
		if slices.Contains(inCats, t) {
			both = append(both, t)
		}
	}
	if len(both) == 0 {
		return nil, httpx.BadRequest("type and category have no type in common")
	}
	return both, nil
}

// Severities reads "severity": minor, moderate, severe, extreme.
func Severities(v url.Values) ([]string, error) {
	return List(v, "severity", func(s string) (string, bool) {
		_, ok := severity.Parse(s)
		return s, ok
	})
}

// Country reads "country" (ISO 3166-1 alpha-2, any case) as upper case;
// "" when absent.
func Country(v url.Values) (string, error) {
	s := strings.ToUpper(v.Get("country"))
	if s != "" && !countryPattern.MatchString(s) {
		return "", httpx.BadRequest("country must be an ISO 3166-1 alpha-2 code")
	}
	return s, nil
}
