package wikidata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
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
	// "former" one (real rows), a Moon coordinate, an unnamed item.
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
	s.endpoint = srv.URL
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
	s.endpoint = failing.URL
	if _, err := s.Poll(context.Background(), domain.Validators{}); err == nil || len(store.bases) != 2 {
		t.Fatalf("a failed class must fail the poll: %v", err)
	}
}

func TestPollRefusesToShrink(t *testing.T) {
	srv := stub(t, "")
	defer srv.Close()
	store := &fakeStore{held: 10}
	s := New(fetch.New(fetch.Options{HostGap: time.Millisecond}), store)
	s.endpoint = srv.URL
	if _, err := s.Poll(context.Background(), domain.Validators{}); err == nil || store.bases != nil {
		t.Fatalf("2 bases against 10 stored must not replace: %v", err)
	}
}

func TestQueryNamesOneClass(t *testing.T) {
	q := Query("Q695850")
	for _, want := range []string{"wdt:P31 wd:Q695850 ;", "ps:P31 wd:Q695850 ;", "wd:Q11639308", `LANG(?label) = "en"`} {
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
