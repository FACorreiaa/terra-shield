// Package countries serves the country views (S2.4): every country with
// its active events at a glance, and one country's summary (what is
// happening, how serious, how well it is covered).
package countries

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/features/events"
	"shield/internal/httpx"
	"shield/internal/server"

	"github.com/labstack/echo/v5"
)

// Repository reads countries. store.CountryRepository is one.
type Repository interface {
	// List returns every country with its active-event stats.
	List(ctx context.Context) ([]domain.CountryStats, error)
	// Summary returns one country, or domain.ErrNotFound.
	Summary(ctx context.Context, iso2 string, since time.Time) (domain.CountrySummary, error)
}

const (
	// maxAge stays below the shortest poll interval (ADR 16).
	maxAge = 15 * time.Second
	// changesWindow is how far back a summary's recent changes reach.
	changesWindow = 24 * time.Hour
	mimeJSON      = "application/json"
)

var iso2Pattern = regexp.MustCompile(`^[A-Z]{2}$`)

// Feature mounts the country routes.
type Feature struct {
	countries Repository // nil when no database is configured (routes command)
	now       func() time.Time
}

// New builds the feature on a Repository.
func New(countries Repository) *Feature {
	return &Feature{countries: countries, now: time.Now}
}

// Name implements server.Feature.
func (*Feature) Name() string { return "countries" }

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/countries", f.list)
	r.V1.GET("/countries/:iso2/summary", f.summary)
	return nil
}

func (f *Feature) list(c *echo.Context) error {
	if f.countries == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	rows, err := f.countries.List(c.Request().Context())
	if err != nil {
		return err
	}
	out := countryList{Countries: make([]Country, 0, len(rows))}
	for i := range rows {
		out.Countries = append(out.Countries, fromStats(&rows[i]))
	}
	body, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return httpx.Cached(c, mimeJSON, body, httpx.ETag(body), maxAge)
}

func (f *Feature) summary(c *echo.Context) error {
	if f.countries == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	iso2 := strings.ToUpper(c.Param("iso2"))
	if !iso2Pattern.MatchString(iso2) {
		return httpx.BadRequest("iso2 must be an ISO 3166-1 alpha-2 code")
	}
	now := f.now().UTC()
	s, err := f.countries.Summary(c.Request().Context(), iso2, now.Add(-changesWindow))
	if errors.Is(err, domain.ErrNotFound) {
		return httpx.NotFound("country not found")
	}
	if err != nil {
		return err
	}
	content := fromSummary(&s)
	etagSource, err := json.Marshal(content)
	if err != nil {
		return err
	}
	body, err := json.Marshal(summaryResponse{summaryContent: content, GeneratedAt: now})
	if err != nil {
		return err
	}
	return httpx.Cached(c, mimeJSON, body, httpx.ETag(etagSource), maxAge)
}

// Country is the wire shape of one country.
type Country struct {
	ISO2         string     `json:"iso2"`
	ISO3         string     `json:"iso3"`
	Name         string     `json:"name"`
	ActiveEvents int        `json:"activeEvents"`
	MaxSeverity  string     `json:"maxSeverity,omitempty"`
	BBox         [4]float64 `json:"bbox"`
}

type countryList struct {
	Countries []Country `json:"countries"`
}

// SeverityCount is one cell of the type × severity matrix.
type SeverityCount struct {
	Type     domain.EventType `json:"type"`
	Severity string           `json:"severity"`
	Count    int              `json:"count"`
}

// Coverage is ADR 7's coverage note.
type Coverage struct {
	OfficialWarnings bool     `json:"officialWarnings"`
	Sources          []string `json:"sources"`
}

// Change is one recent revision of an event in the country.
type Change struct {
	EventID string         `json:"eventId"`
	Title   string         `json:"title"`
	At      time.Time      `json:"at"`
	Changes map[string]any `json:"changes"`
}

// summaryContent identifies a summary's content (the ETag source).
type summaryContent struct {
	Country       Country         `json:"country"`
	Counts        []SeverityCount `json:"counts"`
	Coverage      Coverage        `json:"coverage"`
	Events        []events.Event  `json:"events"`
	RecentChanges []Change        `json:"recentChanges"`
}

type summaryResponse struct {
	summaryContent
	GeneratedAt time.Time `json:"generatedAt"`
}

func fromStats(s *domain.CountryStats) Country {
	return Country{
		ISO2: s.ISO2, ISO3: s.ISO3, Name: s.Name,
		ActiveEvents: s.ActiveEvents, MaxSeverity: s.MaxSeverity, BBox: s.BBox,
	}
}

func fromSummary(s *domain.CountrySummary) summaryContent {
	out := summaryContent{
		Country:       fromStats(&s.Country),
		Counts:        make([]SeverityCount, 0, len(s.Counts)),
		Coverage:      Coverage{OfficialWarnings: s.Coverage.OfficialWarnings, Sources: s.Coverage.Sources},
		Events:        make([]events.Event, 0, len(s.Events)),
		RecentChanges: make([]Change, 0, len(s.RecentChanges)),
	}
	if out.Coverage.Sources == nil {
		out.Coverage.Sources = []string{}
	}
	for _, c := range s.Counts {
		out.Counts = append(out.Counts, SeverityCount(c))
	}
	for i := range s.Events {
		out.Events = append(out.Events, events.FromRecord(&s.Events[i]))
	}
	for _, ch := range s.RecentChanges {
		out.RecentChanges = append(out.RecentChanges, Change{EventID: ch.EventID, Title: ch.Title, At: ch.At.UTC(), Changes: ch.Changes})
	}
	return out
}
