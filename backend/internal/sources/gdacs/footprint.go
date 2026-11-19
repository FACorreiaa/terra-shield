package gdacs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"shield/internal/domain"
	"shield/internal/severity"
)

// footprintClasses is, per GDACS event code, which polygon of the episode's
// geometry collection is the event's footprint:
//
//   - TC: the 60 km/h (gale-force) wind buffer, which covers the whole
//     track including the forecast; per-timestep and stronger-wind buffers
//     are subsets of it.
//   - FL: the affected area (Poly_Global is GloFAS's whole basin).
//   - DR, WF, VO: the affected area.
var footprintClasses = map[string][]string{
	"TC": {"Poly_Green"},
	"FL": {"Poly_Affected"},
	"DR": {"Poly_area"},
	"WF": {"Poly_area"},
	"VO": {"Poly_area", "Poly_Affected"},
}

// needsFootprint: only active Orange and Red alerts. They are few (a
// handful a week) while each geometry is 6–300 KB, and they are the events
// where a wrong country list misleads most. It reads GDACS's own alert, not
// the floored severity: the area floor alone would lift ~150 fires.
func needsFootprint(it *Item) bool {
	return it.Status == domain.StatusActive && it.Alert >= severity.Severe && len(footprintClasses[it.Code]) > 0
}

type geometryCollection struct {
	Features []struct {
		Geometry   json.RawMessage `json:"geometry"`
		Properties struct {
			Class string `json:"Class"`
			Label string `json:"polygonlabel"`
		} `json:"properties"`
	} `json:"features"`
}

// selectFootprint picks the footprint polygon out of a getgeometry
// response; ok is false when the episode has none of the expected shape.
func selectFootprint(code string, body []byte) (json.RawMessage, bool, error) {
	var gc geometryCollection
	if err := json.Unmarshal(body, &gc); err != nil {
		return nil, false, fmt.Errorf("decode geometry: %w", err)
	}
	for _, class := range footprintClasses[code] {
		for _, f := range gc.Features {
			p := f.Properties
			if p.Class != class {
				continue
			}
			// TC: the whole-track buffer is labeled by wind speed; the
			// per-timestep buffers share the class but carry a date label.
			if code == "TC" && !strings.HasSuffix(p.Label, "km/h") {
				continue
			}
			if isPolygonal(f.Geometry) {
				return f.Geometry, true, nil
			}
		}
	}
	return nil, false, nil
}

func isPolygonal(g json.RawMessage) bool {
	var t struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(g, &t) == nil && (t.Type == "Polygon" || t.Type == "MultiPolygon")
}

// applyFootprint replaces an eligible item's point with its footprint. A
// failed fetch is logged and the point kept: one missing polygon must not
// fail the poll that carries every other event.
func (s *Source) applyFootprint(ctx context.Context, it *Item) {
	if !needsFootprint(it) {
		return
	}
	key := it.Code + ":" + it.EventID + ":" + it.EpisodeID
	geom, ok := s.footprints.get(key)
	if !ok {
		var err error
		geom, err = s.fetchFootprint(ctx, it)
		if err != nil {
			s.logger.Warn("gdacs footprint unavailable; keeping point", "event", it.SourceEventID, "error", err)
			return
		}
		s.footprints.put(key, geom) // nil is cached too: "this episode has none"
	}
	if geom != nil {
		it.Geometry = geom
		it.Attrs["footprint"] = true
	}
}

func (s *Source) fetchFootprint(ctx context.Context, it *Item) (json.RawMessage, error) {
	q := url.Values{"eventtype": {it.Code}, "eventid": {it.EventID}, "episodeid": {it.EpisodeID}}
	resp, err := s.client.Get(ctx, s.geometryURL+"?"+q.Encode(), domain.Validators{})
	if err != nil {
		return nil, err
	}
	geom, ok, err := selectFootprint(it.Code, resp.Body)
	if err != nil || !ok {
		return nil, err
	}
	return geom, nil
}

// footprintCache keeps one geometry per event episode, so each is fetched
// once rather than every poll. GDACS bumps the episode when the footprint
// changes.
type footprintCache struct {
	mu sync.Mutex
	m  map[string]json.RawMessage
}

func newFootprintCache() *footprintCache {
	return &footprintCache{m: map[string]json.RawMessage{}}
}

func (c *footprintCache) get(key string) (json.RawMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g, ok := c.m[key]
	return g, ok
}

func (c *footprintCache) put(key string, g json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = g
}

// retain drops entries for episodes no longer in the feed, which bounds the
// cache to the current Orange and Red events.
func (c *footprintCache) retain(items []Item) {
	keep := make(map[string]bool, len(items))
	for i := range items {
		keep[items[i].Code+":"+items[i].EventID+":"+items[i].EpisodeID] = true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if !keep[k] {
			delete(c.m, k)
		}
	}
}
