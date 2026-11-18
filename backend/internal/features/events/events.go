// Package events serves the event API (S2.2): GET /v1/events, the map's
// main query.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/httpx"
	"shield/internal/server"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// Repository reads stored events. store.EventRepository is one.
type Repository interface {
	// List returns events matching q and whether more matched than q.Limit.
	List(ctx context.Context, q domain.EventQuery) ([]domain.EventRecord, bool, error)
	// Detail returns one event, or domain.ErrNotFound.
	Detail(ctx context.Context, id uuid.UUID) (domain.EventDetail, error)
}

const (
	// maxAge stays below the shortest poll interval (ADR 16).
	maxAge = 15 * time.Second
	// clusterBelowZoom: from this zoom up, every event is drawn on its own.
	clusterBelowZoom = 6
	// clusterFetchLimit bounds what one clustered view reads. The world has
	// a few thousand active events at most; beyond this the view is marked
	// truncated rather than read without bound.
	clusterFetchLimit = 10_000

	mimeGeoJSON = "application/geo+json"
	mimeJSON    = "application/json"
)

// Feature mounts the event routes.
type Feature struct {
	events Repository // nil when no database is configured (routes command)
	now    func() time.Time
}

// New builds the feature on a Repository.
func New(events Repository) *Feature {
	return &Feature{events: events, now: time.Now}
}

// Name implements server.Feature.
func (*Feature) Name() string { return "events" }

// Register implements server.Feature.
func (f *Feature) Register(r *server.Routes) error {
	r.V1.GET("/events", f.list)
	r.V1.GET("/events/:id", f.detail)
	return nil
}

func (f *Feature) detail(c *echo.Context) error {
	if f.events == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.BadRequest("id must be a UUID")
	}
	d, err := f.events.Detail(c.Request().Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		return httpx.NotFound("event not found")
	}
	if err != nil {
		return err
	}
	body, err := json.Marshal(toWireDetail(&d, f.now()))
	if err != nil {
		return err
	}
	return httpx.Cached(c, mimeJSON, body, httpx.ETag(body), maxAge)
}

func (f *Feature) list(c *echo.Context) error {
	if f.events == nil {
		return httpx.Abort(http.StatusServiceUnavailable, "database not configured")
	}
	p, err := parseListParams(c.Request().URL.Query())
	if err != nil {
		return err
	}
	c.Response().Header().Add("Vary", "Accept")
	if wantsGeoJSON(c.Request().Header.Get("Accept")) {
		return f.listGeoJSON(c, p)
	}

	q := p.query
	clustered := p.zoom != nil && *p.zoom < clusterBelowZoom
	if clustered {
		q.Limit = clusterFetchLimit
	}
	recs, truncated, err := f.events.List(c.Request().Context(), q)
	if err != nil {
		return err
	}

	page := eventPage{Clusters: []Cluster{}}
	singles := recs
	if clustered {
		var clusters []Cluster
		clusters, singles = cluster(recs, *p.zoom)
		if clusters != nil {
			page.Clusters = clusters
		}
		if len(singles) > p.query.Limit {
			singles, truncated = singles[:p.query.Limit], true
		}
	}
	page.Events = toWireList(singles)
	page.Truncated = truncated

	etagSource, err := json.Marshal(page)
	if err != nil {
		return err
	}
	body, err := json.Marshal(eventList{eventPage: page, GeneratedAt: f.now().UTC()})
	if err != nil {
		return err
	}
	return httpx.Cached(c, mimeJSON, body, httpx.ETag(etagSource), maxAge)
}

func (f *Feature) listGeoJSON(c *echo.Context, p listParams) error {
	q := p.query
	q.Geometry = true
	if p.zoom != nil {
		q.SimplifyDegrees = simplifyTolerance(*p.zoom)
	}
	recs, truncated, err := f.events.List(c.Request().Context(), q)
	if err != nil {
		return err
	}
	fc := featureCollection{Type: "FeatureCollection", Features: make([]feature, 0, len(recs)), Truncated: truncated}
	for i := range recs {
		ev := FromRecord(&recs[i])
		fc.Features = append(fc.Features, feature{Type: "Feature", ID: ev.ID, Geometry: recs[i].Geometry, Properties: ev})
	}
	body, err := json.Marshal(fc)
	if err != nil {
		return err
	}
	return httpx.Cached(c, mimeGeoJSON, body, httpx.ETag(body), maxAge)
}

// simplifyTolerance is half a screen pixel, in degrees, at zoom: detail
// finer than that cannot be drawn.
func simplifyTolerance(zoom int) float64 {
	return 360 / (256 * float64(uint64(1)<<uint(zoom))) / 2
}

// wantsGeoJSON: an explicit GeoJSON media type in Accept. Plain JSON stays
// the default, including for */*.
func wantsGeoJSON(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		mt, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(mt), mimeGeoJSON) {
			return true
		}
	}
	return false
}
