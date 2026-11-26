// Package changes serves what changed over a window (BACKLOG queue item
// 2): events that arrived, ended, escalated or were downgraded, one net
// change per event, with counts that stay complete when the list is cut.
package changes

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"time"

	"shield/internal/domain"
	"shield/internal/features/params"
	"shield/internal/httpx"
	"shield/internal/server"
	"shield/internal/severity"

	"github.com/labstack/echo/v5"
)

// Repository reads changes. store.ChangeRepository is one.
type Repository interface {
	List(ctx context.Context, q domain.ChangeQuery) ([]domain.Change, error)
}

const (
	// DefaultWindow is how far back changes reach without "since".
	DefaultWindow = 24 * time.Hour
	// MaxWindow bounds "since". Resolved events are kept 90 days, so every
	// ending inside it is still there.
	MaxWindow    = 7 * 24 * time.Hour
	defaultLimit = 200
	maxLimit     = 500
	// maxAge stays below the shortest poll interval (ADR 16).
	maxAge   = 15 * time.Second
	mimeJSON = "application/json"
)

// Feature mounts the route.
type Feature struct {
	changes Repository // nil when no database is configured (routes command)
	now     func() time.Time
}

// New builds the feature.
func New(changes Repository) *Feature {
	return &Feature{changes: changes, now: time.Now}
}

// Name implements server.Feature.
func (*Feature) Name() string { return "changes" }

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/changes", f.list)
	return nil
}

// Answer is GET /v1/changes.
type Answer struct {
	Since       time.Time      `json:"since"`
	GeneratedAt time.Time      `json:"generatedAt"`
	Counts      map[string]int `json:"counts"`
	ByType      []TypeCounts   `json:"byType"`
	Changes     []Item         `json:"changes"`
	Truncated   bool           `json:"truncated"`
}

// TypeCounts is one type's changes by kind; zero kinds are omitted.
type TypeCounts struct {
	Type   string         `json:"type"`
	Counts map[string]int `json:"counts"`
}

// Item is one event's change: enough to list and pin it, not the event.
type Item struct {
	EventID        string     `json:"eventId"`
	Change         string     `json:"change"`
	At             time.Time  `json:"at"`
	Type           string     `json:"type"`
	Kind           string     `json:"kind"`
	Severity       string     `json:"severity"`
	FromSeverity   string     `json:"fromSeverity,omitempty"`
	SourceSeverity string     `json:"sourceSeverity,omitempty"`
	Title          string     `json:"title"`
	Source         string     `json:"source"`
	Centroid       [2]float64 `json:"centroid"`
	Countries      []string   `json:"countries"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
}

func (f *Feature) list(c *echo.Context) error {
	if f.changes == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	v := c.Request().URL.Query()
	now := f.now().UTC()
	q := domain.ChangeQuery{Since: now.Add(-DefaultWindow)}
	since, err := params.Time(v, "since")
	if err != nil {
		return err
	}
	if since != nil {
		if since.After(now) || since.Before(now.Add(-MaxWindow)) {
			return httpx.BadRequest("since must be within the last 7 days")
		}
		q.Since = *since
	}
	if q.Types, err = params.Types(v); err != nil {
		return err
	}
	if q.Severities, err = params.Severities(v); err != nil {
		return err
	}
	if q.Country, err = params.Country(v); err != nil {
		return err
	}
	limit := defaultLimit
	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxLimit {
			return httpx.BadRequest("limit must be between 1 and " + strconv.Itoa(maxLimit))
		}
		limit = n
	}

	rows, err := f.changes.List(c.Request().Context(), q)
	if err != nil {
		return err
	}
	body, err := json.Marshal(answer(rows, q.Since, now, limit))
	if err != nil {
		return err
	}
	return httpx.Cached(c, mimeJSON, body, httpx.ETag(body), maxAge)
}

// answer counts every change, then lists the most severe first, newest
// first within a severity, cut at limit.
func answer(rows []domain.Change, since, now time.Time, limit int) Answer {
	a := Answer{Since: since, GeneratedAt: now, Counts: map[string]int{}, ByType: []TypeCounts{}, Changes: []Item{}}
	for _, k := range domain.ChangeKinds() {
		a.Counts[string(k)] = 0
	}
	byType := map[string]map[string]int{}
	for i := range rows {
		r := &rows[i]
		a.Counts[string(r.Change)]++
		t := string(r.Type)
		if byType[t] == nil {
			byType[t] = map[string]int{}
		}
		byType[t][string(r.Change)]++
	}
	for t, counts := range byType {
		a.ByType = append(a.ByType, TypeCounts{Type: t, Counts: counts})
	}
	slices.SortFunc(a.ByType, func(x, y TypeCounts) int { return cmp.Compare(x.Type, y.Type) })

	sorted := slices.Clone(rows)
	slices.SortFunc(sorted, func(x, y domain.Change) int {
		return cmp.Or(
			cmp.Compare(rank(y.Severity), rank(x.Severity)),
			y.At.Compare(x.At),
			cmp.Compare(x.EventID, y.EventID),
		)
	})
	if len(sorted) > limit {
		sorted, a.Truncated = sorted[:limit], true
	}
	for i := range sorted {
		r := &sorted[i]
		countries := r.Countries
		if countries == nil {
			countries = []string{}
		}
		a.Changes = append(a.Changes, Item{
			EventID: r.EventID, Change: string(r.Change), At: r.At, Type: string(r.Type), Kind: string(r.Kind),
			Severity: r.Severity, FromSeverity: r.FromSeverity, SourceSeverity: r.SourceSeverity, Title: r.Title,
			Source: r.SourceID, Centroid: r.Centroid, Countries: countries, StartedAt: r.StartedAt,
		})
	}
	return a
}

func rank(s string) int {
	l, ok := severity.Parse(s)
	if !ok {
		return -1
	}
	return int(l)
}
