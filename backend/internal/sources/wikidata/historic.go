package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
)

// HistoricID is the historic layer's source (L4, ADR 30).
const HistoricID = "wikidata-historic"

// HistoricClass is one Wikidata class on the historic layer.
type HistoricClass struct {
	QID   string
	Label string
	Kind  domain.HistoricKind
	// Base marks the bases layer's classes: only their items that are not
	// on the bases layer (closed, former) belong here, which is decided
	// when reading, so these need the bases layer loaded.
	Base bool
}

// HistoricClasses in priority order: an item in several keeps the first.
// Castles come first because they have their own filter (owner); then
// military heritage, most specific first; the bases layer's classes;
// memorials last, so a fort that is now a museum stays a fort.
//
// Queries take direct instances only (a subclass walk is too slow for the
// service), so the subclasses that hold many items are listed themselves:
// without them Himeji (Japanese castle), Fort Sumter (sea fort) and Tyne
// Cot (Commonwealth war graves cemetery) were missing (2026-11-25).
// "Deserted castle site" (4,207 items, mostly hilltops with little left)
// is left out.
var HistoricClasses = func() []HistoricClass {
	cs := []HistoricClass{
		{"Q23413", "castle", domain.HistoricCastle, false},
		{"Q17715832", "castle ruin", domain.HistoricCastle, false},
		{"Q92062", "motte-and-bailey castle", domain.HistoricCastle, false},
		{"Q92026", "Japanese castle", domain.HistoricCastle, false},
		{"Q91312", "tower house", domain.HistoricCastle, false},
		{"Q7335033", "ringwork castle", domain.HistoricCastle, false},
		{"Q15710038", "hill castle", domain.HistoricCastle, false},
		{"Q32164125", "fortified manor house", domain.HistoricCastle, false},
		{"Q12797", "star fort", domain.HistoricMilitary, false},
		{"Q20097897", "sea fort", domain.HistoricMilitary, false},
		{"Q39535649", "artillery fort", domain.HistoricMilitary, false},
		{"Q3323096", "presidio", domain.HistoricMilitary, false},
		{"Q263274", "kremlin", domain.HistoricMilitary, false},
		{"Q91285", "Martello tower", domain.HistoricMilitary, false},
		{"Q88291", "citadel", domain.HistoricMilitary, false},
		{"Q57831", "fortress", domain.HistoricMilitary, false},
		{"Q1785071", "fort", domain.HistoricMilitary, false},
		{"Q90754", "redoubt", domain.HistoricMilitary, false},
		{"Q82101", "blockhouse", domain.HistoricMilitary, false},
		{"Q89521", "casemate", domain.HistoricMilitary, false},
		{"Q81917", "fortified tower", domain.HistoricMilitary, false},
		{"Q15815716", "high-rise bunker", domain.HistoricMilitary, false},
		{"Q27678942", "deep bunker", domain.HistoricMilitary, false},
		{"Q471359", "tobruk", domain.HistoricMilitary, false},
		{"Q91122", "bunker", domain.HistoricMilitary, false},
		{"Q92069", "pillbox", domain.HistoricMilitary, false},
		{"Q389959", "air-raid shelter", domain.HistoricMilitary, false},
		{"Q131263", "barracks", domain.HistoricMilitary, false},
		{"Q81669", "arsenal", domain.HistoricMilitary, false},
		{"Q744099", "hillfort", domain.HistoricMilitary, false},
		{"Q57821", "fortification", domain.HistoricMilitary, false},
	}
	for _, c := range Classes {
		cs = append(cs, HistoricClass{c.QID, c.Label, domain.HistoricMilitary, true})
	}
	return append(cs,
		HistoricClass{"Q2772772", "military museum", domain.HistoricMemorial, false},
		HistoricClass{"Q14750991", "Commonwealth war graves cemetery", domain.HistoricMemorial, false},
		HistoricClass{"Q19951362", "national necropolis (France)", domain.HistoricMemorial, false},
		HistoricClass{"Q1516659", "United States national cemetery", domain.HistoricMemorial, false},
		HistoricClass{"Q25377388", "military plot (carré militaire)", domain.HistoricMemorial, false},
		HistoricClass{"Q133040655", "military cemetery (Israel)", domain.HistoricMemorial, false},
		HistoricClass{"Q66424110", "military mausoleum", domain.HistoricMemorial, false},
		HistoricClass{"Q1241568", "war cemetery", domain.HistoricMemorial, false},
		HistoricClass{"Q1707610", "military cemetery", domain.HistoricMemorial, false},
	)
}()

// HistoricQuery is the SPARQL for one class: every named item with
// coordinates, closed or not. Castles, the largest class (20,622 items),
// take 10-25 s this way.
func HistoricQuery(classQID string) string {
	return fmt.Sprintf(`SELECT ?b ?c ?label ?desc ?iso WHERE {
  ?b wdt:P31 wd:%s ; wdt:P625 ?c ; rdfs:label ?label . FILTER(LANG(?label) = "en")
  OPTIONAL { ?b schema:description ?desc FILTER(LANG(?desc) = "en") }
  OPTIONAL { ?b wdt:P17/wdt:P297 ?iso }
}`, classQID)
}

// HistoricStore is where the layer is written.
type HistoricStore interface {
	Replace(ctx context.Context, sites []domain.HistoricSite, hints map[string]string, at time.Time) (int, error)
	CountByKind(ctx context.Context) (map[domain.HistoricKind]int, error)
}

// BaseCounter reports whether the bases layer is loaded.
type BaseCounter interface {
	Count(ctx context.Context) (int, error)
}

type historicAnswer struct {
	sites []domain.HistoricSite
	hints map[string]string
}

// Historic is the historic layer's ingest adapter.
type Historic struct {
	q     *pacer[historicAnswer]
	store HistoricStore
	bases BaseCounter
}

// NewHistoric builds the adapter. client should allow QueryTimeout per
// request and bodies of tens of megabytes (castles: 12 MB).
func NewHistoric(client *fetch.Client, store HistoricStore, bases BaseCounter) *Historic {
	return &Historic{q: newPacer[historicAnswer](client), store: store, bases: bases}
}

// Info implements ingest.Source.
func (*Historic) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           HistoricID,
		Name:         "Wikidata (historic sites)",
		URL:          "https://www.wikidata.org/",
		License:      "CC0 1.0",
		Attribution:  "Wikidata",
		PollInterval: PollInterval,
		Enabled:      true,
		NonHazard:    true,
		Scope:        "Forts, castles, bunkers, war cemeteries and other historic sites listed on Wikidata",
		TermsURL:     "https://www.wikidata.org/wiki/Wikidata:Licensing",
		Cadence:      "edited continuously by volunteers; Shield refreshes weekly",
	}
}

// PollTimeout implements ingest.SlowSource: 57 queries of up to a minute,
// each preceded by a wait as long as the last. A full poll measured about
// 12 minutes (2026-11-25, 36 queries); a retry resumes where it failed.
func (*Historic) PollTimeout() time.Duration { return 2 * time.Hour }

// Poll implements ingest.Source. It replaces the whole layer, or nothing.
func (h *Historic) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	// Without the bases layer, every base would read as historic: leave
	// those classes out until it is loaded (a fresh database).
	nBases, err := h.bases.Count(ctx)
	if err != nil {
		return ingest.Result{}, err
	}
	h.q.begin()
	var all []domain.HistoricSite
	hints := map[string]string{}
	seen := map[string]bool{}
	for _, c := range HistoricClasses {
		if c.Base && nBases == 0 {
			continue
		}
		a, err := h.q.run(ctx, c.QID, c.Label, HistoricQuery(c.QID), func(body []byte) (historicAnswer, error) {
			s, hs, err := ParseHistoric(body, c)
			return historicAnswer{s, hs}, err
		})
		if err != nil {
			return ingest.Result{}, err
		}
		for i := range a.sites {
			q := a.sites[i].QID
			if seen[q] {
				continue
			}
			seen[q] = true
			all = append(all, a.sites[i])
			if iso := a.hints[q]; iso != "" {
				hints[q] = iso
			}
		}
	}
	if err := h.checkShrink(ctx, all); err != nil {
		return ingest.Result{}, err
	}
	n, err := h.store.Replace(ctx, all, hints, h.q.now().UTC())
	if err != nil {
		return ingest.Result{}, err
	}
	h.q.finish()
	return ingest.Result{Upserted: n}, nil
}

// checkShrink refuses a poll that would drop more than maxShrink of any
// kind: a class missing from an answer is a failure, not deletions.
func (h *Historic) checkShrink(ctx context.Context, sites []domain.HistoricSite) error {
	held, err := h.store.CountByKind(ctx)
	if err != nil {
		return err
	}
	got := map[domain.HistoricKind]int{}
	for i := range sites {
		got[sites[i].Kind]++
	}
	for _, k := range domain.HistoricKinds {
		if held[k] > 0 && float64(got[k]) < float64(held[k])*(1-maxShrink) {
			return fmt.Errorf("%d %s sites against %d stored: refusing to drop more than %.0f%% in one poll", got[k], k, held[k], maxShrink*100)
		}
	}
	return nil
}

// ParseHistoric reads one class's answer: named sites on Earth, one per
// item, and Wikidata's country per item as a fallback.
func ParseHistoric(body []byte, c HistoricClass) ([]domain.HistoricSite, map[string]string, error) {
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errShape, err)
	}
	if r.Results == nil {
		return nil, nil, errShape
	}
	seen := map[string]bool{}
	hints := map[string]string{}
	var out []domain.HistoricSite
	for _, b := range r.Results.Bindings {
		qid := strings.TrimPrefix(b["b"].Value, "http://www.wikidata.org/entity/")
		if !strings.HasPrefix(qid, "Q") {
			continue
		}
		if iso := b["iso"].Value; len(iso) == 2 && hints[qid] == "" {
			hints[qid] = strings.ToUpper(iso)
		}
		if seen[qid] {
			continue // another coordinate or country: first wins
		}
		name := strings.TrimSpace(b["label"].Value)
		lon, lat, ok := point(b["c"].Value)
		if name == "" || name == qid || !ok {
			continue
		}
		seen[qid] = true
		out = append(out, domain.HistoricSite{
			QID: qid, Name: name, Description: strings.TrimSpace(b["desc"].Value),
			Kind: c.Kind, Class: c.Label, Lon: lon, Lat: lat,
		})
	}
	return out, hints, nil
}
