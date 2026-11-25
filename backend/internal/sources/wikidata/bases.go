// Package wikidata reads the military bases layer (L3, ADR 29) from the
// Wikidata Query Service: installations with coordinates and no recorded
// closure, one query per class.
//
// Wikidata is CC0, so nothing is owed, but every base links to its item
// and the layer credits Wikidata. Wikidata records closures unevenly, so a
// base here is "listed on Wikidata with no recorded closure", never
// "active": items with an end, closure or dissolution date (on the item or
// on its class statement), a closed state of use, or a description that
// says former, historic and the like are left out, as are unnamed items.
// Barracks are left out by default (owner call pending): many are
// historic buildings.
package wikidata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
)

const (
	ID       = "wikidata-bases"
	Endpoint = "https://query.wikidata.org/sparql"
	// PollInterval: bases change rarely, and each poll runs 16 queries
	// against a shared public service.
	PollInterval = 7 * 24 * time.Hour
	// QueryTimeout is above the service's own 60 s limit.
	QueryTimeout = 70 * time.Second
	// maxShrink refuses a poll that would remove more than this share of
	// the stored layer: a partial answer is a failed poll, not deletions.
	maxShrink = 0.2
)

// Class is one Wikidata class the layer takes, in priority order: an item
// in several classes keeps the first (most specific) one.
type Class struct {
	QID   string
	Label string
	Kind  domain.BaseKind
}

// Classes are the installation classes, most specific first.
var Classes = []Class{
	{"Q21193688", "missile base", domain.BaseMissile},
	{"Q331367", "submarine base", domain.BaseNaval},
	{"Q6981985", "naval air base", domain.BaseNaval},
	{"Q1324633", "naval base", domain.BaseNaval},
	{"Q7619063", "naval shore establishment", domain.BaseNaval},
	{"Q133288241", "military radar station", domain.BaseRadar},
	{"Q7373622", "Royal Air Force station", domain.BaseAir},
	{"Q1428515", "Fliegerhorst", domain.BaseAir},
	{"Q695850", "air base", domain.BaseAir},
	{"Q2886620", "Canadian Forces base", domain.BaseGeneral},
	{"Q136644356", "PLA military base", domain.BaseGeneral},
	{"Q2604714", "forward operating base", domain.BaseGeneral},
	{"Q88556", "garrison", domain.BaseGeneral},
	{"Q1321241", "outpost", domain.BaseGeneral},
	{"Q1198413", "military camp", domain.BaseGeneral},
	{"Q245016", "military base", domain.BaseGeneral},
}

// closedStates are state-of-use (P5817) values that mean not in service:
// decommissioned, out of service, permanently closed, abandoned, not in
// operation.
const closedStates = "wd:Q11639308 wd:Q56651571 wd:Q104664889 wd:Q63065035 wd:Q111802839"

// Query is the SPARQL for one class.
func Query(classQID string) string {
	return fmt.Sprintf(`SELECT ?b ?c ?label ?desc ?iso ?opLabel WHERE {
  ?b wdt:P31 wd:%[1]s ; wdt:P625 ?c ; rdfs:label ?label . FILTER(LANG(?label) = "en")
  FILTER NOT EXISTS { ?b wdt:P576|wdt:P3999|wdt:P582|wdt:P730 [] }
  FILTER NOT EXISTS { ?b p:P31 ?st . ?st ps:P31 wd:%[1]s ; pq:P582 [] }
  FILTER NOT EXISTS { ?b wdt:P5817 ?s . VALUES ?s { %[2]s } }
  OPTIONAL { ?b schema:description ?desc FILTER(LANG(?desc) = "en") }
  OPTIONAL { ?b wdt:P17/wdt:P297 ?iso }
  OPTIONAL { ?b wdt:P137 ?op . ?op rdfs:label ?opLabel FILTER(LANG(?opLabel) = "en") }
}`, classQID, closedStates)
}

// Store is where the layer is written.
type Store interface {
	Replace(ctx context.Context, bases []domain.MilitaryBase, hints map[string]string, at time.Time) (int, error)
	Count(ctx context.Context) (int, error)
}

// Source is the ingest adapter.
type Source struct {
	endpoint string
	client   *fetch.Client
	store    Store
	now      func() time.Time
}

// New builds the adapter. client should allow QueryTimeout per request.
func New(client *fetch.Client, store Store) *Source {
	return &Source{endpoint: Endpoint, client: client, store: store, now: time.Now}
}

// Info implements ingest.Source.
func (*Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "Wikidata (military installations)",
		URL:          "https://www.wikidata.org/",
		License:      "CC0 1.0",
		Attribution:  "Wikidata",
		PollInterval: PollInterval,
		Enabled:      true,
		NonHazard:    true,
		Scope:        "Military installations listed on Wikidata with coordinates and no recorded closure",
		TermsURL:     "https://www.wikidata.org/wiki/Wikidata:Licensing",
		Cadence:      "edited continuously by volunteers; Shield refreshes weekly",
	}
}

// PollTimeout implements ingest.SlowSource: 16 queries, up to a minute each.
func (*Source) PollTimeout() time.Duration { return 20 * time.Minute }

// Poll implements ingest.Source. It replaces the whole layer, or nothing.
func (s *Source) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	var all []domain.MilitaryBase
	hints := map[string]string{}
	seen := map[string]bool{}
	for _, c := range Classes {
		resp, err := s.client.Get(ctx, s.endpoint+"?format=json&query="+url.QueryEscape(Query(c.QID)), fetch.Validators{})
		if err != nil {
			return ingest.Result{}, fmt.Errorf("%s: %w", c.Label, err)
		}
		bases, h, err := Parse(resp.Body, c)
		if err != nil {
			return ingest.Result{}, fmt.Errorf("%s: %w", c.Label, err)
		}
		for i := range bases {
			if seen[bases[i].QID] {
				continue // already taken by a more specific class
			}
			seen[bases[i].QID] = true
			all = append(all, bases[i])
			if iso := h[bases[i].QID]; iso != "" {
				hints[bases[i].QID] = iso
			}
		}
	}
	held, err := s.store.Count(ctx)
	if err != nil {
		return ingest.Result{}, err
	}
	if held > 0 && float64(len(all)) < float64(held)*(1-maxShrink) {
		return ingest.Result{}, fmt.Errorf("%d bases against %d stored: refusing to drop more than %.0f%% in one poll", len(all), held, maxShrink*100)
	}
	n, err := s.store.Replace(ctx, all, hints, s.now().UTC())
	if err != nil {
		return ingest.Result{}, err
	}
	return ingest.Result{Upserted: n}, nil
}

type response struct {
	Results *struct {
		Bindings []map[string]struct {
			Value string `json:"value"`
		} `json:"bindings"`
	} `json:"results"`
}

var (
	// notInService catches closures recorded only in prose: a description
	// that starts by saying the place is former, closed and the like ("former
	// German submarine base"), or that places it in a world war, or calls it
	// a museum, internment or prisoner camp. Only the start counts for the
	// first group: "air base in Japan, former Japanese WW2 airfield" is an
	// operating base with a history (Kadena).
	notInService = regexp.MustCompile(`(?i)^\s*(former|formerly|closed|disused|defunct|historic|historical|abandoned|decommissioned|demolished|ruins?)\b|\b(in|during) (the )?(first |second )?world war\b|\bmuseum\b|\binternment\b|\bprisoner`)
	// wktPoint is an Earth point; other globes carry an IRI prefix.
	wktPoint = regexp.MustCompile(`^Point\(\s*(-?[0-9.]+(?:[eE]-?[0-9]+)?)\s+(-?[0-9.]+(?:[eE]-?[0-9]+)?)\s*\)$`)
)

// errShape is a response that is not a SPARQL JSON result.
var errShape = errors.New("not a SPARQL JSON result")

// Parse reads one class's answer: the bases, deduplicated by item, and
// Wikidata's country (ISO 3166-1 alpha-2) per item as a fallback.
func Parse(body []byte, c Class) ([]domain.MilitaryBase, map[string]string, error) {
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errShape, err)
	}
	if r.Results == nil {
		return nil, nil, errShape
	}
	byQID := map[string]*domain.MilitaryBase{}
	var order []string
	hints := map[string]string{}
	dropped := map[string]bool{}
	for _, b := range r.Results.Bindings {
		qid := strings.TrimPrefix(b["b"].Value, "http://www.wikidata.org/entity/")
		if !strings.HasPrefix(qid, "Q") || dropped[qid] {
			continue
		}
		name, desc := strings.TrimSpace(b["label"].Value), strings.TrimSpace(b["desc"].Value)
		if name == "" || name == qid || notInService.MatchString(desc) {
			dropped[qid] = true
			continue
		}
		if iso := b["iso"].Value; len(iso) == 2 && hints[qid] == "" {
			hints[qid] = strings.ToUpper(iso)
		}
		if held := byQID[qid]; held != nil {
			if held.Operator == "" {
				held.Operator = b["opLabel"].Value
			}
			continue // a second coordinate, country or operator: first wins
		}
		lon, lat, ok := point(b["c"].Value)
		if !ok {
			continue
		}
		byQID[qid] = &domain.MilitaryBase{
			QID: qid, Name: name, Description: desc, Kind: c.Kind, Class: c.Label,
			Operator: b["opLabel"].Value, Lon: lon, Lat: lat,
		}
		order = append(order, qid)
	}
	out := make([]domain.MilitaryBase, 0, len(order))
	for _, q := range order {
		out = append(out, *byQID[q])
	}
	return out, hints, nil
}

func point(wkt string) (lon, lat float64, ok bool) {
	m := wktPoint.FindStringSubmatch(strings.TrimSpace(wkt))
	if m == nil {
		return 0, 0, false
	}
	lon, err1 := strconv.ParseFloat(m[1], 64)
	lat, err2 := strconv.ParseFloat(m[2], 64)
	if err1 != nil || err2 != nil || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
		return 0, 0, false
	}
	return lon, lat, true
}
