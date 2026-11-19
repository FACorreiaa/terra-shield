// Package gdacs ingests the Global Disaster Alert and Coordination System
// (UN + European Commission JRC) event feed.
//
// Feed: https://www.gdacs.org/xml/rss.xml, the canonical list of recent
// events with their current alert level. It is chosen over the JSON API
// because the API caps every page at 100 events (wildfires alone exceed
// that, crowding out Orange and Red events) and sends no validators,
// while the RSS file honors If-None-Match, so polling it every 5 minutes
// costs a 304 most of the time (ADR 16).
//
// Scope:
//   - Tropical cyclones, floods, droughts, wildfires, volcanoes.
//   - Earthquakes are skipped: GDACS takes them from USGS/NEIC, which
//     Shield already ingests directly with its PAGER impact alert, so
//     keeping both would put every large quake on the map twice (ADR 5).
//   - Geometry is the event point, except for active Orange and Red events,
//     whose footprint polygon is fetched (see footprint.go) so a drought
//     or flood is assigned every country it covers (ADR 4). GDACS's own
//     affected-country list is kept in attrs either way.
//
// License: CC BY 4.0 (European Union), credit to GDACS required.
package gdacs

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/severity"
)

const (
	// ID is the source key in the sources table.
	ID = "gdacs"
	// FeedURL is the full current-events RSS.
	FeedURL = "https://www.gdacs.org/xml/rss.xml"
	// GeometryURL serves an event episode's footprint polygons.
	GeometryURL = "https://www.gdacs.org/gdacsapi/api/polygons/getgeometry"
	// PollInterval: GDACS rewrites the file every few minutes.
	PollInterval = 5 * time.Minute
)

// Source is the GDACS adapter.
type Source struct {
	feedURL     string
	geometryURL string
	client      *fetch.Client
	store       ingest.EventStore
	footprints  *footprintCache
	logger      *slog.Logger
}

// New builds the adapter on the production feed.
func New(client *fetch.Client, store ingest.EventStore, logger *slog.Logger) *Source {
	return &Source{
		feedURL:     FeedURL,
		geometryURL: GeometryURL,
		client:      client,
		store:       store,
		footprints:  newFootprintCache(),
		logger:      logger,
	}
}

// Info implements ingest.Source.
func (*Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "GDACS (Global Disaster Alert and Coordination System)",
		URL:          "https://www.gdacs.org/",
		License:      "CC BY 4.0 (European Union)",
		Attribution:  "GDACS, a cooperation framework of the United Nations and the European Commission",
		PollInterval: PollInterval,
		Enabled:      true,
	}
}

// Poll implements ingest.Source.
func (s *Source) Poll(ctx context.Context, v domain.Validators) (ingest.Result, error) {
	resp, err := s.client.Get(ctx, s.feedURL, v)
	if err != nil {
		return ingest.Result{}, err
	}
	if resp.NotModified {
		return ingest.Result{Validators: resp.Validators, NotModified: true}, nil
	}
	items, err := Parse(resp.Body)
	if err != nil {
		return ingest.Result{}, err
	}
	events := make([]domain.Event, 0, len(items))
	for i := range items {
		s.applyFootprint(ctx, &items[i])
		events = append(events, items[i].Event)
	}
	s.footprints.retain(items)
	n, err := s.store.Upsert(ctx, ID, events)
	if err != nil {
		return ingest.Result{}, err
	}
	return ingest.Result{Validators: resp.Validators, Upserted: n}, nil
}

const (
	nsGDACS = "http://www.gdacs.org"
	nsGeo   = "http://www.w3.org/2003/01/geo/wgs84_pos#"
)

type valueText struct {
	Unit  string `xml:"unit,attr"`
	Value string `xml:"value,attr"`
	Text  string `xml:",chardata"`
}

type item struct {
	Description   string    `xml:"description"`
	Link          string    `xml:"link"`
	GUID          string    `xml:"guid"`
	IsCurrent     string    `xml:"http://www.gdacs.org iscurrent"`
	FromDate      string    `xml:"http://www.gdacs.org fromdate"`
	ToDate        string    `xml:"http://www.gdacs.org todate"`
	DateModified  string    `xml:"http://www.gdacs.org datemodified"`
	EventType     string    `xml:"http://www.gdacs.org eventtype"`
	AlertLevel    string    `xml:"http://www.gdacs.org alertlevel"`
	AlertScore    string    `xml:"http://www.gdacs.org alertscore"`
	EventName     string    `xml:"http://www.gdacs.org eventname"`
	EventID       string    `xml:"http://www.gdacs.org eventid"`
	EpisodeID     string    `xml:"http://www.gdacs.org episodeid"`
	Severity      valueText `xml:"http://www.gdacs.org severity"`
	Population    valueText `xml:"http://www.gdacs.org population"`
	Vulnerability valueText `xml:"http://www.gdacs.org vulnerability"`
	ISO3          string    `xml:"http://www.gdacs.org iso3"`
	Country       string    `xml:"http://www.gdacs.org country"`
	Glide         string    `xml:"http://www.gdacs.org glide"`
	BBox          string    `xml:"http://www.gdacs.org bbox"`
	CAP           string    `xml:"http://www.gdacs.org cap"`
	Point         struct {
		Lat  string `xml:"http://www.w3.org/2003/01/geo/wgs84_pos# lat"`
		Long string `xml:"http://www.w3.org/2003/01/geo/wgs84_pos# long"`
	} `xml:"http://www.w3.org/2003/01/geo/wgs84_pos# Point"`
}

type feed struct {
	Items []item `xml:"channel>item"`
}

// types maps GDACS event codes to Shield's. EQ is deliberately absent.
var types = map[string]domain.EventType{
	"TC": domain.TypeTropicalCyclone,
	"FL": domain.TypeFlood,
	"DR": domain.TypeDrought,
	"WF": domain.TypeWildfire,
	"VO": domain.TypeVolcano,
}

// Item is one normalized GDACS event plus what the footprint lookup needs.
type Item struct {
	domain.Event
	Code      string // GDACS event type code: TC, FL, DR, WF, VO
	EventID   string
	EpisodeID string
	Alert     severity.Level // GDACS's own impact alert, before any physical floor
}

// Parse normalizes the GDACS RSS feed. Items of unknown or skipped types,
// or without a usable id, point or alert level, are dropped.
func Parse(body []byte) ([]Item, error) {
	var f feed
	// The file starts with a UTF-8 BOM, which encoding/xml rejects.
	if err := xml.Unmarshal(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")), &f); err != nil {
		return nil, fmt.Errorf("gdacs: decode: %w", err)
	}
	var items []Item
	for i := range f.Items {
		if it, ok := normalize(&f.Items[i]); ok {
			items = append(items, it)
		}
	}
	return items, nil
}

func normalize(it *item) (Item, bool) {
	code := strings.TrimSpace(it.EventType)
	eventID := strings.TrimSpace(it.EventID)
	typ, ok := types[code]
	if !ok || eventID == "" {
		return Item{}, false
	}
	alert, ok := severity.GDACSAlert(it.AlertLevel)
	if !ok {
		return Item{}, false
	}
	level, basis := physicalFloor(typ, alert, it.Severity)
	lat, errLat := strconv.ParseFloat(strings.TrimSpace(it.Point.Lat), 64)
	lon, errLon := strconv.ParseFloat(strings.TrimSpace(it.Point.Long), 64)
	if errLat != nil || errLon != nil {
		return Item{}, false
	}

	started := parseTime(it.FromDate)
	latest := parseTime(it.ToDate)
	modified := parseTime(it.DateModified)
	if modified == nil {
		return Item{}, false
	}
	observed := *modified
	if latest != nil {
		observed = *latest
	}

	status, ended := domain.StatusActive, (*time.Time)(nil)
	if strings.EqualFold(strings.TrimSpace(it.IsCurrent), "false") {
		status, ended = domain.StatusResolved, latest
	}

	countries := splitCountries(it.Country)
	attrs := map[string]any{
		"alertLevel": strings.TrimSpace(it.AlertLevel),
		"eventId":    eventID,
		"episodeId":  strings.TrimSpace(it.EpisodeID),
		"point":      []float64{lon, lat}, // GDACS's reference position
		"countries":  countries,           // GDACS's own list: display only, never used for assignment (ADR 4)
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(it.AlertScore), 64); err == nil {
		attrs["alertScore"] = v
	}
	attrs["severityBasis"] = basis
	if t := strings.TrimSpace(it.Severity.Text); t != "" {
		attrs["severityText"] = t
	}
	if v, err := strconv.ParseFloat(it.Severity.Value, 64); err == nil && it.Severity.Unit != "" {
		attrs["severityValue"] = v
		attrs["severityUnit"] = it.Severity.Unit
	}
	if t := strings.TrimSpace(it.Population.Text); t != "" {
		attrs["populationText"] = t
	}
	if t := strings.TrimSpace(it.Vulnerability.Text); t != "" {
		attrs["vulnerability"] = t
	}
	if name := strings.TrimSpace(it.EventName); name != "" {
		attrs["eventName"] = name
	}
	if g := strings.TrimSpace(it.Glide); g != "" {
		attrs["glide"] = g
	}
	if bb := parseBBox(it.BBox); bb != nil {
		attrs["bbox"] = bb
	}
	if c := strings.TrimSpace(it.CAP); c != "" {
		attrs["capUrl"] = httpsURL(c)
	}

	sourceSeverity := strings.TrimSpace(it.AlertLevel) + " alert"
	if t := strings.TrimSpace(it.Severity.Text); t != "" && !noMagnitude.MatchString(t) {
		sourceSeverity += ", " + t
	}

	event := domain.Event{
		SourceEventID:  code + eventID,
		Type:           typ,
		Kind:           domain.KindObserved,
		Status:         status,
		Severity:       level.String(),
		SourceSeverity: sourceSeverity,
		Title:          title(typ, strings.TrimSpace(it.EventName), countries),
		Summary:        tidy(it.Description),
		Geometry:       domain.PointGeometry(lon, lat),
		StartedAt:      started,
		ObservedAt:     observed,
		UpdatedAt:      *modified,
		EndedAt:        ended,
		URL:            httpsURL(strings.TrimSpace(it.Link)),
		Attrs:          attrs,
	}
	return Item{Event: event, Code: code, EventID: eventID, EpisodeID: strings.TrimSpace(it.EpisodeID), Alert: alert}, true
}

var labels = map[domain.EventType]string{
	domain.TypeTropicalCyclone: "Tropical cyclone",
	domain.TypeFlood:           "Flood",
	domain.TypeDrought:         "Drought",
	domain.TypeWildfire:        "Wildfire",
	domain.TypeVolcano:         "Volcanic activity",
}

// title is short and uniform: "Tropical cyclone XAVIER-26",
// "Drought: East Africa, Central Sahel-2026", "Flood in Myanmar",
// "Wildfire in Brazil, Bolivia and 2 more".
func title(typ domain.EventType, name string, countries []string) string {
	label := labels[typ]
	switch {
	case name != "" && typ == domain.TypeDrought:
		return label + ": " + name
	case name != "":
		return label + " " + name
	case len(countries) == 0:
		return label
	case len(countries) <= 2:
		return label + " in " + strings.Join(countries, ", ")
	}
	return fmt.Sprintf("%s in %s and %d more", label, strings.Join(countries[:2], ", "), len(countries)-2)
}

func splitCountries(s string) []string {
	out := []string{}
	for _, c := range strings.Split(s, ",") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// parseBBox turns GDACS's "lonmin lonmax latmin latmax" into GeoJSON order
// [west, south, east, north].
func parseBBox(s string) []float64 {
	f := strings.Fields(s)
	if len(f) != 4 {
		return nil
	}
	v := make([]float64, 4)
	for i, x := range f {
		n, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return nil
		}
		v[i] = n
	}
	return []float64{v[0], v[2], v[1], v[3]}
}

func parseTime(s string) *time.Time {
	t, err := time.Parse(time.RFC1123, strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}

// httpsURL upgrades GDACS's http:// links; the site serves https.
func httpsURL(s string) string {
	if strings.HasPrefix(s, "http://") {
		return "https://" + strings.TrimPrefix(s, "http://")
	}
	return s
}

// physicalFloor lifts the impact alert to the event's physical size where
// GDACS reports one (owner, 2026-11-19; docs/severity.md): a cyclone's wind
// on the Saffir-Simpson bands, a wildfire's burned area on the same table
// as every other fire source. basis names what decided: "alert", "wind" or
// "area".
func physicalFloor(typ domain.EventType, alert severity.Level, sv valueText) (severity.Level, string) {
	v, err := strconv.ParseFloat(strings.TrimSpace(sv.Value), 64)
	if err != nil {
		return alert, "alert"
	}
	var floor severity.Level
	basis := ""
	switch unit := strings.ToLower(strings.TrimSpace(sv.Unit)); {
	case typ == domain.TypeTropicalCyclone && unit == "km/h":
		floor, basis = severity.CycloneWindKmh(v), "wind"
	case typ == domain.TypeWildfire && unit == "ha":
		floor, basis = severity.WildfireAcres(v*severity.AcresPerHectare), "area"
	default:
		return alert, "alert"
	}
	if floor > alert {
		return floor, basis
	}
	return alert, "alert"
}

// noMagnitude is GDACS's placeholder severity text for floods and
// droughts, which have no magnitude: "Magnitude 0".
var noMagnitude = regexp.MustCompile(`^Magnitude 0\s*$`)

var (
	spaces        = regexp.MustCompile(`[ \t]{2,}`)
	spaceBeforePu = regexp.MustCompile(` +([.,;:])`)
)

// tidy cleans GDACS's templated descriptions ("started in Australia,  until
// 15/11/2026." or "1219 displaced .") without changing their words.
func tidy(s string) string {
	s = spaces.ReplaceAllString(strings.TrimSpace(s), " ")
	return spaceBeforePu.ReplaceAllString(s, "$1")
}
