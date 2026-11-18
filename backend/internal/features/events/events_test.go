package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"shield/internal/config"
	"shield/internal/domain"
	"shield/internal/server"

	"github.com/google/uuid"
)

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func rec(id string, sev string, lon, lat float64) domain.EventRecord {
	return domain.EventRecord{
		Event: domain.Event{
			SourceEventID: id, Type: domain.TypeEarthquake, Kind: domain.KindObserved, Status: domain.StatusActive,
			Severity: sev, Title: id, ObservedAt: now, UpdatedAt: now,
			Attrs:    map[string]any{"mag": 5.0},
			Geometry: json.RawMessage(`{"type":"Point","coordinates":[0,0]}`),
		},
		ID: "id-" + id, SourceID: "usgs", Latitude: lat, Longitude: lon, FetchedAt: now,
	}
}

// fakeLister records the last query and honors its limit; detail serves
// one stored detail by id.
type fakeLister struct {
	recs   []domain.EventRecord
	err    error
	got    domain.EventQuery
	detail *domain.EventDetail
}

func (f *fakeLister) Detail(_ context.Context, id uuid.UUID) (domain.EventDetail, error) {
	if f.err != nil {
		return domain.EventDetail{}, f.err
	}
	if f.detail == nil || f.detail.ID != id.String() {
		return domain.EventDetail{}, domain.ErrNotFound
	}
	return *f.detail, nil
}

func (f *fakeLister) List(_ context.Context, q domain.EventQuery) ([]domain.EventRecord, bool, error) {
	f.got = q
	if len(f.recs) > q.Limit {
		return f.recs[:q.Limit], true, f.err
	}
	return f.recs, false, f.err
}

func get(t *testing.T, l Repository, target string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	f := New(l)
	f.now = func() time.Time { return now }
	srv, err := server.New(d, []server.Feature{f})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	return w
}

func TestParseListParams(t *testing.T) {
	p, err := parseListParams(url.Values{
		"bbox": {"170,-20,-170,-10"}, "zoom": {"4"}, "type": {"earthquake, tsunami,,earthquake"},
		"severity": {"severe,extreme"}, "kind": {"warning"}, "status": {"resolved"}, "source": {"usgs"},
		"country": {"pt"}, "since": {"2026-11-15T00:00:00+01:00"}, "until": {"2026-11-16T00:00:00Z"}, "limit": {"20"},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := p.query
	if *p.zoom != 4 || q.BBox.MinLon != 170 || q.BBox.MaxLon != -170 || len(q.Types) != 2 || q.Severities[1] != "extreme" ||
		q.Kinds[0] != domain.KindWarning || q.Status != domain.StatusResolved || q.Sources[0] != "usgs" ||
		q.Country != "PT" || !q.Since.Equal(time.Date(2026, 11, 14, 23, 0, 0, 0, time.UTC)) || q.Limit != 20 {
		t.Fatalf("%+v zoom %v", q, *p.zoom)
	}

	defaults, _ := parseListParams(url.Values{})
	if defaults.query.Status != domain.StatusActive || defaults.query.Limit != 500 || defaults.zoom != nil {
		t.Fatalf("defaults %+v", defaults)
	}

	for name, v := range map[string]url.Values{
		"bbox arity":      {"bbox": {"1,2,3"}},
		"bbox lat order":  {"bbox": {"0,10,1,5"}},
		"bbox range":      {"bbox": {"-181,0,0,1"}},
		"zoom":            {"zoom": {"23"}},
		"type":            {"type": {"quake"}},
		"severity case":   {"severity": {"Severe"}},
		"kind":            {"kind": {"rumor"}},
		"status":          {"status": {"all"}},
		"source":          {"source": {"US GS"}},
		"country":         {"country": {"PRT"}},
		"since":           {"since": {"yesterday"}},
		"since >= until":  {"since": {"2026-11-16T00:00:00Z"}, "until": {"2026-11-16T00:00:00Z"}},
		"limit":           {"limit": {"2001"}},
		"too many values": {"source": {"s1,s2,s3,s4,s5,s6,s7,s8,s9,s10,s11,s12,s13,s14,s15,s16,s17,s18,s19,s20,s21"}},
	} {
		if _, err := parseListParams(v); err == nil {
			t.Errorf("%s: accepted %v", name, v)
		}
	}
}

func TestListJSON(t *testing.T) {
	offshore := rec("off", "minor", -10.5, 38.5)
	offshore.Countries = []domain.EventCountry{{ISO2: "PT", Relation: domain.RelationNearby, DistanceKm: 61.4}}
	eonet := rec("fire", "moderate", -120, 38)
	eonet.Attrs = map[string]any{"severityProvisional": true, "magnitudeValue": 2379.0}
	eonet.Countries = []domain.EventCountry{{ISO2: "US", Relation: domain.RelationWithin}}
	l := &fakeLister{recs: []domain.EventRecord{eonet, offshore}}

	w := get(t, l, "/v1/events?type=wildfire,earthquake", nil)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json" ||
		w.Header().Get("Cache-Control") != "public, max-age=15" || w.Header().Get("ETag") == "" || !slices.Contains(w.Header().Values("Vary"), "Accept") {
		t.Fatalf("%d %v %s", w.Code, w.Header(), w.Body)
	}
	if len(l.got.Types) != 2 || l.got.Geometry {
		t.Fatalf("query %+v", l.got)
	}
	var body struct {
		Events      []map[string]any `json:"events"`
		Clusters    []any            `json:"clusters"`
		Truncated   bool             `json:"truncated"`
		GeneratedAt time.Time        `json:"generatedAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Events) != 2 || body.Clusters == nil || body.Truncated || !body.GeneratedAt.Equal(now) {
		t.Fatalf("%s", w.Body)
	}
	fire, off := body.Events[0], body.Events[1]
	if fire["severityProvisional"] != true || fire["attrs"].(map[string]any)["severityProvisional"] != nil {
		t.Errorf("provisional not lifted: %v", fire)
	}
	if _, ok := off["severityProvisional"]; ok {
		t.Errorf("false provisional should be omitted: %v", off)
	}
	if _, ok := fire["countries"].([]any)[0].(map[string]any)["distanceKm"]; ok {
		t.Errorf("within must not carry distanceKm: %v", fire["countries"])
	}
	if c := off["countries"].([]any)[0].(map[string]any); c["relation"] != "nearby" || c["distanceKm"] != 61.4 {
		t.Errorf("nearby %v", c)
	}
	if off["centroid"].(map[string]any)["latitude"] != 38.5 || off["source"] != "usgs" || off["id"] != "id-off" {
		t.Errorf("event %v", off)
	}
	if eonet.Attrs["severityProvisional"] != true {
		t.Error("FromRecord mutated the record's attrs")
	}

	// Revalidation: same content → 304, even though generatedAt moves.
	again := get(t, l, "/v1/events?type=wildfire,earthquake", http.Header{"If-None-Match": {w.Header().Get("ETag")}})
	if again.Code != http.StatusNotModified {
		t.Fatalf("revalidate %d", again.Code)
	}
}

func TestListClustersAtLowZoom(t *testing.T) {
	l := &fakeLister{recs: []domain.EventRecord{
		rec("a", "extreme", 10.0, 45.0),
		rec("far", "severe", -70, -30),
		rec("b", "minor", 10.1, 45.1),
		rec("c", "moderate", 10.2, 44.9),
	}}

	w := get(t, l, "/v1/events?zoom=3&limit=1", nil)
	if w.Code != http.StatusOK || l.got.Limit != clusterFetchLimit {
		t.Fatalf("%d limit %d", w.Code, l.got.Limit)
	}
	var body struct {
		Events    []Event   `json:"events"`
		Clusters  []Cluster `json:"clusters"`
		Truncated bool      `json:"truncated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Clusters) != 1 || len(body.Events) != 1 || body.Events[0].ID != "id-far" || body.Truncated {
		t.Fatalf("%s", w.Body)
	}
	c := body.Clusters[0]
	if c.Count != 3 || c.MaxSeverity != "extreme" || c.Types["earthquake"] != 3 || len(c.Severities) != 3 ||
		c.Severities["extreme"] != 1 || c.Severities["moderate"] != 1 ||
		c.BBox != [4]float64{10.0, 44.9, 10.2, 45.1} || c.Centroid.Longitude < 10.09 || c.Centroid.Longitude > 10.11 {
		t.Fatalf("cluster %+v", c)
	}

	// From zoom 6 nothing clusters, and limit applies to the query.
	w = get(t, l, "/v1/events?zoom=12&limit=2", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.Clusters) != 0 || len(body.Events) != 2 || !body.Truncated || l.got.Limit != 2 {
		t.Fatalf("zoom 12: %s err %v", w.Body, err)
	}
}

func TestListGeoJSON(t *testing.T) {
	l := &fakeLister{recs: []domain.EventRecord{rec("a", "minor", 1, 2)}}
	w := get(t, l, "/v1/events?zoom=2", http.Header{"Accept": {"application/geo+json;q=1, application/json;q=0.5"}})
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/geo+json" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	if !l.got.Geometry || l.got.SimplifyDegrees != simplifyTolerance(2) || l.got.Limit != 500 {
		t.Fatalf("query %+v", l.got)
	}
	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Type       string          `json:"type"`
			ID         string          `json:"id"`
			Geometry   json.RawMessage `json:"geometry"`
			Properties Event           `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &fc); err != nil || fc.Type != "FeatureCollection" || len(fc.Features) != 1 {
		t.Fatalf("%s err %v", w.Body, err)
	}
	if f := fc.Features[0]; f.Type != "Feature" || f.ID != "id-a" || string(f.Geometry) != `{"type":"Point","coordinates":[0,0]}` || f.Properties.Title != "a" {
		t.Fatalf("feature %+v", f)
	}
}

func TestListErrors(t *testing.T) {
	if w := get(t, &fakeLister{}, "/v1/events?bbox=nope", nil); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "bbox") {
		t.Fatalf("bad param: %d %s", w.Code, w.Body)
	}
	if w := get(t, &fakeLister{err: errors.New("db down")}, "/v1/events", nil); w.Code != http.StatusInternalServerError {
		t.Fatalf("store error: %d", w.Code)
	}
	if w := get(t, nil, "/v1/events", nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("no database: %d", w.Code)
	}
}

func TestMercatorAndCells(t *testing.T) {
	if x, y := mercatorPx(0, 0, 0); x != 128 || y < 127.999 || y > 128.001 {
		t.Fatalf("origin %v %v", x, y)
	}
	if _, y := mercatorPx(0, 90, 0); y < -0.001 {
		t.Fatalf("pole not clamped: %v", y)
	}
	// Either side of the antimeridian never shares a cell.
	clusters, singles := cluster([]domain.EventRecord{rec("e", "minor", 179.9, 0), rec("w", "minor", -179.9, 0)}, 0)
	if len(clusters) != 0 || len(singles) != 2 {
		t.Fatalf("antimeridian merged: %v", clusters)
	}
	if tol := simplifyTolerance(0); tol < 0.70 || tol > 0.71 {
		t.Fatalf("tolerance %v", tol)
	}
}

func TestDetail(t *testing.T) {
	id := uuid.MustParse("6f1c1f5e-8a54-4a0e-9b5e-2f0c3c7c1a11")
	r := rec("q1", "severe", -8, 39)
	r.ID = id.String()
	r.Attrs = map[string]any{"severityProvisional": true}
	r.Geometry = json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}`)
	last := now.Add(-time.Minute)
	d := &domain.EventDetail{
		EventRecord: r,
		Source: domain.SourceStatus{
			SourceInfo:    domain.SourceInfo{ID: "usgs", Name: "USGS", Attribution: "USGS", PollInterval: time.Minute, Enabled: true},
			LastSuccessAt: &last,
		},
		Revisions: []domain.Revision{{At: now, Changes: map[string]any{"severity": map[string]any{"from": "minor", "to": "severe"}}}},
	}
	l := &fakeLister{detail: d}

	w := get(t, l, "/v1/events/"+id.String(), nil)
	if w.Code != http.StatusOK || w.Header().Get("ETag") == "" || w.Header().Get("Cache-Control") != "public, max-age=15" {
		t.Fatalf("%d %v %s", w.Code, w.Header(), w.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["id"] != id.String() || body["severityProvisional"] != true || body["title"] != "q1" {
		t.Errorf("event fields not flattened: %v", body)
	}
	if g := body["geometry"].(map[string]any); g["type"] != "Polygon" {
		t.Errorf("geometry %v", g)
	}
	if s := body["sourceInfo"].(map[string]any); s["id"] != "usgs" || s["pollIntervalSeconds"] != 60.0 || s["stale"] != false {
		t.Errorf("sourceInfo %v", s)
	}
	revs := body["revisions"].([]any)
	if len(revs) != 1 || revs[0].(map[string]any)["changes"].(map[string]any)["severity"] == nil {
		t.Errorf("revisions %v", revs)
	}

	if w := get(t, l, "/v1/events/"+id.String(), http.Header{"If-None-Match": {w.Header().Get("ETag")}}); w.Code != http.StatusNotModified {
		t.Errorf("revalidate: %d", w.Code)
	}
	if w := get(t, l, "/v1/events/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad id: %d", w.Code)
	}
	if w := get(t, l, "/v1/events/"+uuid.NewString(), nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown id: %d", w.Code)
	}
	if w := get(t, &fakeLister{err: errors.New("db down")}, "/v1/events/"+id.String(), nil); w.Code != http.StatusInternalServerError {
		t.Errorf("store error: %d", w.Code)
	}
	if w := get(t, nil, "/v1/events/"+id.String(), nil); w.Code != http.StatusServiceUnavailable {
		t.Errorf("no database: %d", w.Code)
	}
}
