package wikidata

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
)

func binding(qid, label, wkt, desc, iso string) string {
	b := fmt.Sprintf(`{"b":{"value":"http://www.wikidata.org/entity/%s"},"c":{"value":%q},"label":{"value":%q}`, qid, wkt, label)
	if desc != "" {
		b += fmt.Sprintf(`,"desc":{"value":%q}`, desc)
	}
	if iso != "" {
		b += fmt.Sprintf(`,"iso":{"value":%q}`, iso)
	}
	return b + "}"
}

func answer(bs ...string) string { return `{"results":{"bindings":[` + strings.Join(bs, ",") + `]}}` }

var castleClass = HistoricClasses[0]

func TestParseHistoric(t *testing.T) {
	body := answer(
		binding("Q1", "Castelo de São Jorge", "Point(-9.1335 38.7139)", "castle in Lisbon", "pt"),
		binding("Q1", "Castelo de São Jorge", "Point(-9.2 38.8)", "", "PT"), // second coordinate
		binding("Q2", "Q2", "Point(1 1)", "", ""),                           // unnamed
		binding("Q3", "Moon Castle", "<http://www.wikidata.org/entity/Q405> Point(1 1)", "", ""),
		binding("Q4", "Former Castle", "Point(2 50)", "former castle, now ruins", ""), // kept: history is the point
	)
	sites, hints, err := ParseHistoric([]byte(body), castleClass)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || sites[0].QID != "Q1" || sites[1].QID != "Q4" {
		t.Fatalf("%+v", sites)
	}
	s := sites[0]
	if s.Lon != -9.1335 || s.Kind != domain.HistoricCastle || s.Class != "castle" || s.Description != "castle in Lisbon" || hints["Q1"] != "PT" {
		t.Errorf("%+v %v", s, hints)
	}
	if _, _, err := ParseHistoric([]byte(`<html>timeout</html>`), castleClass); err == nil {
		t.Error("a non-JSON answer must fail")
	}
}

type fakeHistoricStore struct {
	mu    sync.Mutex
	sites []domain.HistoricSite
	held  map[domain.HistoricKind]int
}

func (f *fakeHistoricStore) Replace(_ context.Context, s []domain.HistoricSite, _ map[string]string, _ time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sites = s
	f.held = map[domain.HistoricKind]int{}
	for i := range s {
		f.held[s[i].Kind]++
	}
	return len(s), nil
}

func (f *fakeHistoricStore) CountByKind(context.Context) (map[domain.HistoricKind]int, error) {
	return f.held, nil
}

type baseCount int

func (b baseCount) Count(context.Context) (int, error) { return int(b), nil }

// historicStub answers: Q10 is a castle and a fort (castle wins); Q11 a
// fort; Q12 an air base (a bases-layer class); Q13 a war cemetery.
func historicStub(t *testing.T, asked map[string]int, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	by := map[string]string{
		"Q23413":   answer(binding("Q10", "Fort Castle", "Point(1 50)", "", "")),
		"Q1785071": answer(binding("Q10", "Fort Castle", "Point(1 50)", "", ""), binding("Q11", "Old Fort", "Point(2 50)", "", "")),
		"Q695850":  answer(binding("Q12", "RAF Old Field", "Point(0 52)", "", "GB")),
		"Q1241568": answer(binding("Q13", "War Cemetery", "Point(3 50)", "", "")),
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		for qid, body := range by {
			if strings.Contains(q, "wd:"+qid+" ") {
				mu.Lock()
				asked[qid]++
				mu.Unlock()
				_, _ = w.Write([]byte(body))
				return
			}
		}
		_, _ = w.Write([]byte(answer()))
	}))
}

func newTestHistoric(url string, store HistoricStore, bases BaseCounter) *Historic {
	h := NewHistoric(fetch.New(fetch.Options{HostGap: time.Millisecond}), store, bases)
	h.q.endpoint, h.q.gate = url, &gate{}
	h.q.pause = func(context.Context, time.Duration) error { return nil }
	return h
}

func TestHistoricPoll(t *testing.T) {
	var mu sync.Mutex
	asked := map[string]int{}
	srv := historicStub(t, asked, &mu)
	defer srv.Close()
	store := &fakeHistoricStore{}

	// No bases layer yet: its classes are left out.
	h := newTestHistoric(srv.URL, store, baseCount(0))
	if _, err := h.Poll(context.Background(), domain.Validators{}); err != nil {
		t.Fatal(err)
	}
	if asked["Q695850"] != 0 || len(store.sites) != 3 {
		t.Fatalf("without bases: asked air base %d times, %d sites", asked["Q695850"], len(store.sites))
	}

	h = newTestHistoric(srv.URL, store, baseCount(3000))
	res, err := h.Poll(context.Background(), domain.Validators{})
	if err != nil || res.Upserted != 4 {
		t.Fatalf("%+v %v", res, err)
	}
	got := map[string]domain.HistoricSite{}
	for _, s := range store.sites {
		got[s.QID] = s
	}
	if got["Q10"].Kind != domain.HistoricCastle || got["Q10"].Class != "castle" {
		t.Errorf("a castle that is also a fort is a castle: %+v", got["Q10"])
	}
	if got["Q12"].Kind != domain.HistoricMilitary || got["Q12"].Class != "air base" {
		t.Errorf("a bases-layer class item: %+v", got["Q12"])
	}
	if got["Q13"].Kind != domain.HistoricMemorial {
		t.Errorf("war cemetery: %+v", got["Q13"])
	}
}

func TestHistoricRefusesToShrinkAKind(t *testing.T) {
	var mu sync.Mutex
	srv := historicStub(t, map[string]int{}, &mu)
	defer srv.Close()
	// Plenty stored overall, but the castles would drop from 10 to 1.
	store := &fakeHistoricStore{held: map[domain.HistoricKind]int{domain.HistoricCastle: 10, domain.HistoricMilitary: 2}}
	h := newTestHistoric(srv.URL, store, baseCount(3000))
	if _, err := h.Poll(context.Background(), domain.Validators{}); err == nil || store.sites != nil {
		t.Fatalf("castles 10 -> 1 must be refused: %v", err)
	}
}

func TestHistoricClasses(t *testing.T) {
	seen := map[string]bool{}
	bases := 0
	for _, c := range HistoricClasses {
		if seen[c.QID] {
			t.Errorf("class %s listed twice", c.QID)
		}
		seen[c.QID] = true
		if c.Base {
			bases++
		}
		if _, ok := domain.ParseHistoricKind(string(c.Kind)); !ok {
			t.Errorf("class %s has kind %q", c.QID, c.Kind)
		}
	}
	if bases != len(Classes) || HistoricClasses[0].Kind != domain.HistoricCastle {
		t.Errorf("bases classes %d of %d; first %+v", bases, len(Classes), HistoricClasses[0])
	}
	if q := HistoricQuery("Q23413"); strings.Contains(q, "P582") || !strings.Contains(q, "wdt:P31 wd:Q23413 ;") {
		t.Errorf("historic query must be the lean one:\n%s", q)
	}
}
