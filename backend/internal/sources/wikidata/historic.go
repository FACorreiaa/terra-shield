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
		{"Q20034791", "defensive tower", domain.HistoricMilitary, false},
		{"Q947103", "watchtower", domain.HistoricMilitary, false},
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
		// General landmarks (L4 step 2, owner 2026-11-25: "Landmarks",
		// plus mausoleums, basilicas, Roman monuments and Shinto shrines).
		// Subclasses come before their parent so the item keeps the more
		// precise class. Left out as bulk imports rather than landmarks
		// (few heritage-listed): plain church buildings, mosques, Spanish
		// casas consistoriales, sector lights, dovecotes, water towers,
		// gurdwaras; and excavation sites (Q839954, 62k), the owner's call.
		HistoricClass{"Q16560", "palace", domain.HistoricGeneral, false},
		HistoricClass{"Q2651004", "palazzo", domain.HistoricGeneral, false},
		HistoricClass{"Q15848826", "city palace", domain.HistoricGeneral, false},
		HistoricClass{"Q53536964", "royal palace", domain.HistoricGeneral, false},
		HistoricClass{"Q5383482", "episcopal palace", domain.HistoricGeneral, false},
		HistoricClass{"Q751876", "château", domain.HistoricGeneral, false},
		HistoricClass{"Q16823155", "Schloss", domain.HistoricGeneral, false},
		HistoricClass{"Q1424449", "hunting lodge", domain.HistoricGeneral, false},
		HistoricClass{"Q2977", "cathedral", domain.HistoricGeneral, false},
		HistoricClass{"Q56242215", "Catholic cathedral", domain.HistoricGeneral, false},
		HistoricClass{"Q56242225", "Eastern Orthodox cathedral", domain.HistoricGeneral, false},
		HistoricClass{"Q2577114", "co-cathedral", domain.HistoricGeneral, false},
		HistoricClass{"Q124936", "major basilica", domain.HistoricGeneral, false},
		HistoricClass{"Q2713379", "papal basilica", domain.HistoricGeneral, false},
		HistoricClass{"Q120560", "minor basilica", domain.HistoricGeneral, false},
		HistoricClass{"Q163687", "basilica", domain.HistoricGeneral, false},
		HistoricClass{"Q160742", "abbey", domain.HistoricGeneral, false},
		HistoricClass{"Q817056", "Benedictine abbey", domain.HistoricGeneral, false},
		HistoricClass{"Q91315817", "Cistercian monastery", domain.HistoricGeneral, false},
		HistoricClass{"Q2750108", "priory", domain.HistoricGeneral, false},
		HistoricClass{"Q20672229", "friary", domain.HistoricGeneral, false},
		HistoricClass{"Q11784935", "Eastern Orthodox monastery", domain.HistoricGeneral, false},
		HistoricClass{"Q54074585", "Tibetan Buddhist monastery", domain.HistoricGeneral, false},
		HistoricClass{"Q1701174", "monastery ruins", domain.HistoricGeneral, false},
		HistoricClass{"Q96371565", "former monastery", domain.HistoricGeneral, false},
		HistoricClass{"Q44613", "monastery", domain.HistoricGeneral, false},
		HistoricClass{"Q867143", "Roman temple", domain.HistoricGeneral, false},
		HistoricClass{"Q267596", "ancient Greek temple", domain.HistoricGeneral, false},
		HistoricClass{"Q618618", "temple of Confucius", domain.HistoricGeneral, false},
		HistoricClass{"Q2680845", "Chinese temple", domain.HistoricGeneral, false},
		HistoricClass{"Q2736554", "candi", domain.HistoricGeneral, false},
		HistoricClass{"Q427287", "wat", domain.HistoricGeneral, false},
		HistoricClass{"Q199451", "pagoda", domain.HistoricGeneral, false},
		HistoricClass{"Q180987", "stupa", domain.HistoricGeneral, false},
		HistoricClass{"Q5393308", "Buddhist temple", domain.HistoricGeneral, false},
		HistoricClass{"Q842402", "Hindu temple", domain.HistoricGeneral, false},
		HistoricClass{"Q44539", "temple", domain.HistoricGeneral, false},
		HistoricClass{"Q845945", "Shinto shrine", domain.HistoricGeneral, false},
		HistoricClass{"Q34627", "synagogue", domain.HistoricGeneral, false},
		HistoricClass{"Q162875", "mausoleum", domain.HistoricGeneral, false},
		HistoricClass{"Q175582", "Egyptian pyramid", domain.HistoricGeneral, false},
		HistoricClass{"Q12516", "pyramid", domain.HistoricGeneral, false},
		HistoricClass{"Q7362268", "Roman amphitheatre", domain.HistoricGeneral, false}, //nolint:misspell // Wikidata's English label
		HistoricClass{"Q54831", "amphitheatre", domain.HistoricGeneral, false},         //nolint:misspell // Wikidata's English label
		HistoricClass{"Q19757", "Roman theatre", domain.HistoricGeneral, false},        //nolint:misspell // Wikidata's English label
		HistoricClass{"Q474", "aqueduct", domain.HistoricGeneral, false},
		HistoricClass{"Q143912", "triumphal arch", domain.HistoricGeneral, false},
		HistoricClass{"Q82117", "city gate", domain.HistoricGeneral, false},
		HistoricClass{"Q16748868", "city walls", domain.HistoricGeneral, false},
		HistoricClass{"Q879050", "manor house", domain.HistoricGeneral, false},
		HistoricClass{"Q21092604", "manor house", domain.HistoricGeneral, false},
		HistoricClass{"Q16974307", "dwór", domain.HistoricGeneral, false},
		HistoricClass{"Q1408475", "fortified house", domain.HistoricGeneral, false},
		HistoricClass{"Q569879", "Ansitz", domain.HistoricGeneral, false},
		HistoricClass{"Q1523545", "seigniorial home", domain.HistoricGeneral, false},
		HistoricClass{"Q16884952", "country house", domain.HistoricGeneral, false},
		HistoricClass{"Q1802963", "mansion", domain.HistoricGeneral, false},
		HistoricClass{"Q1655611", "plantation house", domain.HistoricGeneral, false},
		HistoricClass{"Q2927789", "buitenplaats", domain.HistoricGeneral, false},
		HistoricClass{"Q2087181", "historic house museum", domain.HistoricGeneral, false},
		HistoricClass{"Q543654", "Rathaus", domain.HistoricGeneral, false},
		HistoricClass{"Q65177565", "former town hall", domain.HistoricGeneral, false},
		HistoricClass{"Q25550691", "city hall", domain.HistoricGeneral, false},
		HistoricClass{"Q39715", "lighthouse", domain.HistoricGeneral, false},
		HistoricClass{"Q200334", "bell tower", domain.HistoricGeneral, false},
		HistoricClass{"Q853854", "clock tower", domain.HistoricGeneral, false},
		HistoricClass{"Q1440300", "observation tower", domain.HistoricGeneral, false},
		HistoricClass{"Q44844", "Bismarck tower", domain.HistoricGeneral, false},
		HistoricClass{"Q12518", "tower", domain.HistoricGeneral, false},
		HistoricClass{"Q180997", "post mill", domain.HistoricGeneral, false},
		HistoricClass{"Q1534870", "smock mill", domain.HistoricGeneral, false},
		HistoricClass{"Q819774", "smock mill on a hill", domain.HistoricGeneral, false},
		HistoricClass{"Q647704", "tower mill", domain.HistoricGeneral, false},
		HistoricClass{"Q38720", "windmill", domain.HistoricGeneral, false},
	)
}()

// WorldHeritageQuery asks for every UNESCO World Heritage item with
// coordinates, with its selection criteria, or its parent's for a part of a
// serial site (1,731 of the 1,986 items without criteria of their own).
const WorldHeritageQuery = `SELECT ?b ?c ?label ?desc ?iso ?cr WHERE {
  ?b wdt:P1435 wd:Q9259 ; wdt:P625 ?c ; rdfs:label ?label . FILTER(LANG(?label) = "en")
  OPTIONAL { ?b schema:description ?desc FILTER(LANG(?desc) = "en") }
  OPTIONAL { ?b wdt:P17/wdt:P297 ?iso }
  OPTIONAL { { ?b wdt:P2614 ?cr } UNION { ?b wdt:P361 ?p . ?p wdt:P2614 ?cr } }
}`

// worldHeritageKey names the World Heritage answer in the pacer.
const worldHeritageKey = "world-heritage"

// naturalCriteria are World Heritage selection criteria (vii) to (x).
var naturalCriteria = map[string]bool{"Q23038981": true, "Q23038983": true, "Q23038985": true, "Q23038986": true}

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
	wh    map[string]bool // World Heritage items, the WorldHeritageQuery answer only
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
		Scope:        "Forts, castles, war cemeteries, palaces, cathedrals, temples and other historic sites and landmarks listed on Wikidata, with World Heritage Sites",
		TermsURL:     "https://www.wikidata.org/wiki/Wikidata:Licensing",
		Cadence:      "edited continuously by volunteers; Shield refreshes weekly",
	}
}

// PollAttempts implements ingest.PatientSource: a retry resumes.
func (*Historic) PollAttempts() int { return pollAttempts }

// UseEndpoint sends the queries to another SPARQL endpoint than the
// Wikidata Query Service (a mirror, in development).
func (h *Historic) UseEndpoint(u string) { h.q.endpoint = u }

// PollTimeout implements ingest.SlowSource: 133 queries (132 classes and
// World Heritage) of up to a minute, each preceded by a wait as long as
// the last. 56 queries took about 12 minutes (2026-11-25); a retry
// resumes where it failed.
func (*Historic) PollTimeout() time.Duration { return 2 * time.Hour }

// Poll implements ingest.Source. It replaces the whole layer, or nothing.
func (h *Historic) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	// Without the bases layer, every base would read as historic, and a
	// poll that left the base classes out would stand for a week. On a
	// fresh database both polls start together: wait without a query (a
	// snooze, which costs no attempt) until bases has loaded.
	nBases, err := h.bases.Count(ctx)
	if err != nil {
		return ingest.Result{}, err
	}
	if nBases == 0 {
		return ingest.Result{}, errNoBases
	}
	h.q.begin()
	var all []domain.HistoricSite
	hints := map[string]string{}
	seen := map[string]bool{}
	for _, c := range HistoricClasses {
		a, err := h.q.run(ctx, c.QID, c.Label, HistoricQuery(c.QID), func(body []byte) (historicAnswer, error) {
			s, hs, err := ParseHistoric(body, c)
			return historicAnswer{sites: s, hints: hs}, err
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
	// World Heritage: flag the sites already here, then add the cultural
	// ones no class took (owner, 2026-11-25), as general landmarks.
	wa, err := h.q.run(ctx, worldHeritageKey, "World Heritage Sites", WorldHeritageQuery, func(body []byte) (historicAnswer, error) {
		s, hs, wh, err := ParseWorldHeritage(body)
		return historicAnswer{sites: s, hints: hs, wh: wh}, err
	})
	if err != nil {
		return ingest.Result{}, err
	}
	for i := range all {
		all[i].WorldHeritage = wa.wh[all[i].QID]
	}
	for i := range wa.sites {
		q := wa.sites[i].QID
		if seen[q] {
			continue
		}
		seen[q] = true
		all = append(all, wa.sites[i])
		if iso := wa.hints[q]; iso != "" {
			hints[q] = iso
		}
	}
	if err := h.checkShrink(ctx, all); err != nil {
		h.q.finish() // a retry must ask again, not replay the short answer
		return ingest.Result{}, err
	}
	n, err := h.store.Replace(ctx, all, hints, h.q.now().UTC())
	if err != nil {
		return ingest.Result{}, err
	}
	h.q.finish()
	return ingest.Result{Upserted: n}, nil
}

// errNoBases: the historic layer is the complement of the bases layer, so it
// waits for that one to load (a cold bases poll took 7 to 21 minutes).
var errNoBases = &ingest.NotReady{After: 5 * time.Minute, Reason: "bases layer not loaded yet: historic sites wait for it"}

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

// ParseWorldHeritage reads the WorldHeritageQuery answer: every World
// Heritage item's QID (for the flag), and as sites the named ones on Earth
// that are not purely natural, as general landmarks. An empty answer is an
// error: the list never shrinks to nothing, and the flag would vanish.
func ParseWorldHeritage(body []byte) ([]domain.HistoricSite, map[string]string, map[string]bool, error) {
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", errShape, err)
	}
	if r.Results == nil || len(r.Results.Bindings) == 0 {
		return nil, nil, nil, fmt.Errorf("%w: no World Heritage Sites", errShape)
	}
	type item struct {
		site              domain.HistoricSite
		ok                bool
		cultural, natural bool
	}
	items := map[string]*item{}
	var order []string
	hints := map[string]string{}
	for _, b := range r.Results.Bindings {
		qid := strings.TrimPrefix(b["b"].Value, "http://www.wikidata.org/entity/")
		if !strings.HasPrefix(qid, "Q") {
			continue
		}
		it := items[qid]
		if it == nil {
			it = &item{}
			items[qid] = it
			order = append(order, qid)
			name := strings.TrimSpace(b["label"].Value)
			if lon, lat, ok := point(b["c"].Value); ok && name != "" && name != qid {
				it.ok = true
				it.site = domain.HistoricSite{
					QID: qid, Name: name, Description: strings.TrimSpace(b["desc"].Value),
					Kind: domain.HistoricGeneral, Class: "World Heritage Site", Lon: lon, Lat: lat, WorldHeritage: true,
				}
			}
		}
		if iso := b["iso"].Value; len(iso) == 2 && hints[qid] == "" {
			hints[qid] = strings.ToUpper(iso)
		}
		if cr := strings.TrimPrefix(b["cr"].Value, "http://www.wikidata.org/entity/"); cr != "" {
			if naturalCriteria[cr] {
				it.natural = true
			} else {
				it.cultural = true
			}
		}
	}
	wh := make(map[string]bool, len(items))
	var out []domain.HistoricSite
	for _, qid := range order {
		it := items[qid]
		wh[qid] = true
		if it.ok && (it.cultural || !it.natural) {
			out = append(out, it.site)
		}
	}
	return out, hints, wh, nil
}
