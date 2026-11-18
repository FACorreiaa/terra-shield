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

// Types reads "type": known event types.
func Types(v url.Values) ([]domain.EventType, error) {
	return List(v, "type", func(s string) (domain.EventType, bool) {
		return domain.EventType(s), domain.EventType(s).Valid()
	})
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
