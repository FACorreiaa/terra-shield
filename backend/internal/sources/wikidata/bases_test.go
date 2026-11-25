package wikidata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/submarine.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var submarine = Class{"Q331367", "submarine base", domain.BaseNaval}

func TestParse(t *testing.T) {
	bases, hints, err := Parse(fixture(t), submarine)
	if err != nil {
		t.Fatal(err)
	}
	// Kept: Point Loma and Kings Bay. Left out: a World War II base and a
	// "former" one (real rows), a Moon coordinate, an unnamed item, and one
	// whose class membership ended (on its second row).
	if len(bases) != 2 || bases[0].QID != "Q6981762" || bases[1].QID != "Q11790255" {
		t.Fatalf("%+v", bases)
	}
	pl := bases[0]
	if pl.Name != "Naval Base Point Loma" || pl.Lon != -117.245 || pl.Lat != 32.6769 || pl.Kind != domain.BaseNaval ||
		pl.Class != "submarine base" || pl.Operator != "United States Navy" || !strings.Contains(pl.Description, "San Diego") {
		t.Errorf("first row wins, second coordinate ignored: %+v", pl)
	}
	if hints["Q6981762"] != "US" || hints["Q321306"] != "" {
		t.Errorf("hints %v", hints)
	}
}

func TestNotInService(t *testing.T) {
	for desc, out := range map[string]bool{
		"former German submarine base and bunker in Norway":                   true,
		"German U-boat base in World War II":                                  true,
		"Closed airfield in Kent":                                             true,
		"aviation museum and former RAF station":                              true,
		"United States Air Force base in Japan, former Japanese WW2 airfield": false, // Kadena
		"base of the United States Navy in Camden County, Georgia":            false,
		"a former U.S. Army base southwest of Pleiku in central Vietnam.":     true,
		"site of former military camp":                                        true,
		"World War II airfield complex in Libya":                              true,
		"Second World War coastal battery for Perth":                          true,
		"ruined castle in Germany":                                            true,
		"Sengoku period castle ruins in Yonezawa":                             true,
		"United States historic site in Alabama":                              true,
		"Civil War historic location":                                         true,
		"local authority in former German Cameroon":                           true,
		"airbase and former Royal Air Force station in Wiltshire, England":    false, // Boscombe Down
		"place in Virginia listed on National Register of Historic Places":    false, // Fort Myer
		"Military base in the Syrian desert, formerly used by the coalition":  false, // Al-Tanf
		"": false,
	} {
		if got := notInService.MatchString(desc); got != out {
			t.Errorf("%q: left out %v, want %v", desc, got, out)
		}
	}
}

func TestParseFailsClosed(t *testing.T) {
	for _, body := range []string{`{}`, `<html>upstream request timeout</html>`, `{"results":`} {
		if _, _, err := Parse([]byte(body), submarine); err == nil {
			t.Errorf("%q parsed", body)
		}
	}
	if b, _, err := Parse([]byte(`{"results":{"bindings":[]}}`), submarine); err != nil || len(b) != 0 {
		t.Errorf("an empty class is an answer: %v %v", b, err)
	}
}

type fakeStore struct {
	mu    sync.Mutex
	held  int
	bases []domain.MilitaryBase
	hints map[string]string
}

func (f *fakeStore) Replace(_ context.Context, b []domain.MilitaryBase, h map[string]string, _ time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bases, f.hints, f.held = b, h, len(b)
	return len(b), nil
}

func (f *fakeStore) Count(context.Context) (int, error) { return f.held, nil }

// stub answers each class's query: the fixture for submarine bases, Point
// Loma again as a missile base (a more specific class), nothing else.
func stub(t *testing.T, fail string) *httptest.Server {
	t.Helper()
	body := fixture(t)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		switch {
		case fail != "" && strings.Contains(q, "wd:"+fail+" "):
			http.Error(w, "upstream request timeout", http.StatusGatewayTimeout)
		case strings.Contains(q, "wd:Q331367 "):
			_, _ = w.Write(body)
		case strings.Contains(q, "wd:Q21193688 "):
			_, _ = w.Write([]byte(`{"results":{"bindings":[{"b":{"value":"http://www.wikidata.org/entity/Q6981762"},
				"c":{"value":"Point(-117.2 32.7)"},"label":{"value":"Naval Base Point Loma"}}]}}`))
		default:
			_, _ = w.Write([]byte(`{"results":{"bindings":[]}}`))
		}
	}))
}

func TestPoll(t *testing.T) {
	srv := stub(t, "")
	defer srv.Close()
	store := &fakeStore{}
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store)
	s.q.endpoint = srv.URL
	s.q.pause = func(context.Context, time.Duration) error { return nil }
	res, err := s.Poll(context.Background(), domain.Validators{})
	if err != nil || res.Upserted != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if store.bases[0].QID != "Q6981762" || store.bases[0].Kind != domain.BaseMissile || store.bases[0].Lon != -117.2 {
		t.Errorf("an item in two classes keeps the more specific: %+v", store.bases[0])
	}
	if store.hints["Q11790255"] != "US" {
		t.Errorf("hints %v", store.hints)
	}

	// One class failing fails the poll and keeps the stored layer.
	failing := stub(t, "Q695850")
	defer failing.Close()
	s.q.endpoint = failing.URL
	if _, err := s.Poll(context.Background(), domain.Validators{}); err == nil || len(store.bases) != 2 {
		t.Fatalf("a failed class must fail the poll: %v", err)
	}
}

func TestPollRefusesToShrink(t *testing.T) {
	srv := stub(t, "")
	defer srv.Close()
	store := &fakeStore{held: 10}
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store)
	s.q.endpoint = srv.URL
	s.q.pause = func(context.Context, time.Duration) error { return nil }
	if _, err := s.Poll(context.Background(), domain.Validators{}); err == nil || store.bases != nil {
		t.Fatalf("2 bases against 10 stored must not replace: %v", err)
	}
}

// A wait past maxHold is handed to River as a snooze; a refusal with no
// Retry-After holds the gate for refusedHold; a query waiting for the gate
// stops when its context ends.
func TestGateHoldLimits(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	status := http.StatusBadGateway
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != http.StatusOK {
			http.Error(w, "busy", status)
			return
		}
		_, _ = w.Write([]byte(`{"results":{"bindings":[]}}`))
	}))
	defer srv.Close()
	g := newGate()
	var waits []time.Duration
	p := newPacer[int](fetch.New(fetch.Options{HostGap: time.Millisecond}))
	p.endpoint, p.gate, p.now = srv.URL, g, func() time.Time { return now }
	p.pause = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	p.begin()
	defer p.finish()
	parse := func([]byte) (int, error) { return 0, nil }

	if _, err := p.run(context.Background(), "1", "one", "q1", parse); err == nil {
		t.Fatal("a 502 must fail the query")
	}
	status = http.StatusOK
	if _, err := p.run(context.Background(), "2", "two", "q2", parse); err != nil {
		t.Fatal(err)
	}
	if len(waits) != 1 || waits[0] != refusedHold {
		t.Fatalf("after a bare 502 the gate holds %v: %v", refusedHold, waits)
	}

	g.ended, g.prev = now, time.Hour
	_, err := p.run(context.Background(), "3", "three", "q3", parse)
	var se *fetch.StatusError
	if !errors.As(err, &se) || se.RetryAfter != time.Hour || len(waits) != 1 {
		t.Fatalf("a wait past maxHold must come back as a snooze: %v, waits %v", err, waits)
	}

	<-g.slot // another query holds the gate
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.run(ctx, "4", "four", "q4", parse); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting for the gate must stop on cancel: %v", err)
	}
	g.slot <- struct{}{}
}

// Queries declare their prefixes, so any standard SPARQL endpoint answers.
func TestQueriesDeclarePrefixes(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("query")
		_, _ = w.Write([]byte(`{"results":{"bindings":[]}}`))
	}))
	defer srv.Close()
	p := newPacer[int](fetch.New(fetch.Options{HostGap: time.Millisecond}))
	p.endpoint, p.gate = srv.URL, newGate()
	if _, err := p.run(context.Background(), "k", "label", HistoricQuery("Q1"), func([]byte) (int, error) { return 0, nil }); err != nil {
		t.Fatal(err)
	}
	for _, pfx := range []string{"wd", "wdt", "rdfs", "schema", "p", "ps", "pq"} {
		if !strings.Contains(got, "PREFIX "+pfx+": <") {
			t.Errorf("query does not declare %s:", pfx)
		}
	}
	if !strings.HasSuffix(got, HistoricQuery("Q1")) {
		t.Error("the query itself must follow the prefixes unchanged")
	}
}

func TestSourcesArePatient(t *testing.T) {
	var _ ingest.PatientSource = (*Source)(nil)
	var _ ingest.PatientSource = (*Historic)(nil)
	var _ ingest.SlowSource = (*Historic)(nil)
}

func TestQueryNamesOneClass(t *testing.T) {
	q := Query("Q695850")
	for _, want := range []string{"wdt:P31 wd:Q695850 ;", "ps:P31 wd:Q695850 ; pq:P582 ?ended", "?ended WHERE", "wd:Q11639308", `LANG(?label) = "en"`} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %q", want)
		}
	}
	seen := map[string]bool{}
	for _, c := range Classes {
		if seen[c.QID] {
			t.Errorf("class %s listed twice", c.QID)
		}
		seen[c.QID] = true
	}
}

// A retried poll asks only for the classes it has not answered yet.
func TestPollResumes(t *testing.T) {
	var mu sync.Mutex
	asked := map[string]int{}
	failOnce := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		var cls string
		for _, c := range Classes {
			if strings.Contains(q, "wd:"+c.QID+" ") {
				cls = c.QID
				break
			}
		}
		mu.Lock()
		asked[cls]++
		fail := cls == "Q695850" && failOnce
		if fail {
			failOnce = false
		}
		mu.Unlock()
		if fail {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"results":{"bindings":[]}}`))
	}))
	defer srv.Close()
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), &fakeStore{})
	s.q.endpoint = srv.URL
	s.q.pause = func(context.Context, time.Duration) error { return nil }

	if _, err := s.Poll(context.Background(), domain.Validators{}); err == nil {
		t.Fatal("the air base class failed: the poll must fail")
	}
	if _, err := s.Poll(context.Background(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	for i, c := range Classes {
		want := 1
		if c.QID == "Q695850" {
			want = 2
		}
		if asked[c.QID] != want {
			t.Errorf("class %d %s asked %d times, want %d", i, c.Label, asked[c.QID], want)
		}
	}
	// A completed poll forgets its answers: the next one asks again.
	if _, err := s.Poll(context.Background(), domain.Validators{}); err != nil || asked["Q245016"] != 2 {
		t.Fatalf("after a completed poll: %v, military base asked %d times", err, asked["Q245016"])
	}
}

func TestPacerForgetsOldAnswers(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	p := newPacer[int](nil)
	p.now = func() time.Time { return now }
	p.begin()
	p.done["x"] = 1
	now = now.Add(resumeFor - time.Minute)
	p.begin()
	if _, ok := p.done["x"]; !ok {
		t.Fatal("an answer within resumeFor must be kept")
	}
	now = now.Add(2 * time.Minute)
	p.begin()
	if _, ok := p.done["x"]; ok {
		t.Fatal("an answer older than resumeFor must be dropped")
	}
}

// The bases query is pinned: a change must be deliberate, and timed against
// the service (ADR 29: the larger classes take 5-16 s).
func TestBasesQueryPinned(t *testing.T) {
	want := `SELECT ?b ?c ?label ?desc ?iso ?opLabel ?ended WHERE {
  ?b wdt:P31 wd:Q695850 ; wdt:P625 ?c ; rdfs:label ?label . FILTER(LANG(?label) = "en")
  FILTER NOT EXISTS { ?b wdt:P576|wdt:P3999|wdt:P582|wdt:P730 [] }
  OPTIONAL { ?b p:P31 ?st . ?st ps:P31 wd:Q695850 ; pq:P582 ?ended }
  FILTER NOT EXISTS { ?b wdt:P5817 ?s . VALUES ?s { wd:Q11639308 wd:Q56651571 wd:Q104664889 wd:Q63065035 wd:Q111802839 } }
  OPTIONAL { ?b schema:description ?desc FILTER(LANG(?desc) = "en") }
  OPTIONAL { ?b wdt:P17/wdt:P297 ?iso }
  OPTIONAL { ?b wdt:P137 ?op . ?op rdfs:label ?opLabel FILTER(LANG(?opLabel) = "en") }
}`
	if got := Query("Q695850"); got != want {
		t.Fatalf("bases query changed:\n%s", got)
	}
}

// The gate makes each query wait until the previous one's duration has
// passed since it ended, across every source in the process.
func TestGatePaces(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		now = now.Add(10 * time.Second) // each query takes 10 s
		_, _ = w.Write([]byte(`{"results":{"bindings":[]}}`))
	}))
	defer srv.Close()
	g := newGate()
	var waits []time.Duration
	mk := func() *pacer[int] {
		p := newPacer[int](fetch.New(fetch.Options{HostGap: time.Millisecond}))
		p.endpoint, p.gate, p.now = srv.URL, g, clock
		p.pause = func(_ context.Context, d time.Duration) error { waits = append(waits, d); now = now.Add(d); return nil }
		p.begin()
		return p
	}
	a, b := mk(), mk() // two sources sharing the service
	parse := func([]byte) (int, error) { return 0, nil }
	ctx := context.Background()
	if _, err := a.run(ctx, "1", "one", "q1", parse); err != nil {
		t.Fatal(err)
	}
	if _, err := b.run(ctx, "2", "two", "q2", parse); err != nil {
		t.Fatal(err)
	}
	now = now.Add(4 * time.Second)
	if _, err := a.run(ctx, "3", "three", "q3", parse); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := b.run(ctx, "4", "four", "q4", parse); err != nil {
		t.Fatal(err)
	}
	// No wait before the first; 10 s before the second (other source);
	// 6 s before the third (4 s had passed); none after a quiet minute.
	if len(waits) != 2 || waits[0] != 10*time.Second || waits[1] != 6*time.Second {
		t.Fatalf("waits %v", waits)
	}
}

// A 429 holds every source until its Retry-After.
func TestGateHonoursRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	limited := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if limited {
			limited = false
			w.Header().Set("Retry-After", "120")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"results":{"bindings":[]}}`))
	}))
	defer srv.Close()
	g := newGate()
	var waits []time.Duration
	mk := func() *pacer[int] {
		p := newPacer[int](fetch.New(fetch.Options{HostGap: time.Millisecond}))
		p.endpoint, p.gate, p.now = srv.URL, g, func() time.Time { return now }
		p.pause = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
		p.begin()
		return p
	}
	a, b := mk(), mk()
	parse := func([]byte) (int, error) { return 0, nil }
	if _, err := a.run(context.Background(), "1", "one", "q1", parse); err == nil {
		t.Fatal("a 429 must fail the query")
	}
	if _, err := b.run(context.Background(), "2", "two", "q2", parse); err != nil {
		t.Fatal(err)
	}
	if len(waits) != 1 || waits[0] != 2*time.Minute {
		t.Fatalf("the other source must wait out Retry-After: %v", waits)
	}
}
