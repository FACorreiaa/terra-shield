package events

import (
	"net/url"
	"regexp"
	"strconv"

	"shield/internal/domain"
	"shield/internal/features/params"
	"shield/internal/httpx"
)

const (
	defaultLimit = 500
	maxLimit     = 2000
)

var sourcePattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

type listParams struct {
	query domain.EventQuery
	zoom  *int
}

// parseListParams validates every query parameter of GET /v1/events; a
// bad one is a 400 naming it, never silently ignored.
func parseListParams(v url.Values) (listParams, error) {
	p := listParams{query: domain.EventQuery{Status: domain.StatusActive, Limit: defaultLimit}}
	q := &p.query

	var err error
	if q.BBox, err = params.BBox(v); err != nil {
		return p, err
	}
	if p.zoom, err = params.Zoom(v); err != nil {
		return p, err
	}

	if q.Types, err = params.Types(v); err != nil {
		return p, err
	}
	if q.Severities, err = params.Severities(v); err != nil {
		return p, err
	}
	if q.Kinds, err = params.List(v, "kind", func(s string) (domain.Kind, bool) {
		return domain.Kind(s), domain.Kind(s).Valid()
	}); err != nil {
		return p, err
	}
	if q.Sources, err = params.List(v, "source", func(s string) (string, bool) {
		return s, sourcePattern.MatchString(s)
	}); err != nil {
		return p, err
	}

	if s := v.Get("status"); s != "" {
		if !domain.Status(s).Valid() {
			return p, httpx.BadRequest("status must be active or resolved")
		}
		q.Status = domain.Status(s)
	}
	if q.Country, err = params.Country(v); err != nil {
		return p, err
	}
	if q.Since, err = params.Time(v, "since"); err != nil {
		return p, err
	}
	if q.Until, err = params.Time(v, "until"); err != nil {
		return p, err
	}
	if q.Since != nil && q.Until != nil && !q.Since.Before(*q.Until) {
		return p, httpx.BadRequest("since must be before until")
	}
	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxLimit {
			return p, httpx.BadRequest("limit must be an integer from 1 to 2000")
		}
		q.Limit = n
	}
	return p, nil
}
