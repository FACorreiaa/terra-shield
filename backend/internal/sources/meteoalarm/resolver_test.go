package meteoalarm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"shield/internal/domain"
)

type fakeAreaStore struct {
	codes      map[domain.AreaCode][]string
	codeCalls  int
	unionCalls int
	lastCodes  []string
	lastPolys  int
}

func (f *fakeAreaStore) Codes(_ context.Context, gs []domain.AreaCode) (map[domain.AreaCode][]string, error) {
	f.codeCalls++
	out := map[domain.AreaCode][]string{}
	for _, g := range gs {
		if c, ok := f.codes[g]; ok {
			out[g] = c
		}
	}
	return out, nil
}

func (f *fakeAreaStore) Union(_ context.Context, codes []string, polys []json.RawMessage) (json.RawMessage, error) {
	f.unionCalls++
	f.lastCodes, f.lastPolys = codes, len(polys)
	return json.RawMessage(`{"type":"MultiPolygon","coordinates":[],"n":"` + strings.Join(codes, ",") + `"}`), nil
}

func TestResolverResolvesAliasesAndCaches(t *testing.T) {
	be := domain.AreaCode{Type: domain.AreaTypeEMMA, Value: "BE001"}
	nuts := domain.AreaCode{Type: "NUTS2", Value: "BE34"}
	fr := domain.AreaCode{Type: "NUTS3", Value: "FR712"}
	store := &fakeAreaStore{codes: map[domain.AreaCode][]string{be: {"BE001"}, nuts: {"BE001"}, fr: {"FR026"}}}
	r := NewResolver(store)
	ctx := context.Background()
	poly := json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}`)
	areas := []Area{{Codes: []domain.AreaCode{fr}}, {Codes: []domain.AreaCode{be}}, {Codes: []domain.AreaCode{nuts}}, {Polygons: []json.RawMessage{poly}}}

	g, ok, err := r.Resolve(ctx, areas)
	if err != nil || !ok || !strings.Contains(string(g), "BE001,FR026") || store.lastPolys != 1 {
		t.Fatalf("geom %s ok %v err %v polys %d", g, ok, err, store.lastPolys)
	}
	// Same areas in another order: served from the cache.
	reordered := []Area{areas[3], areas[1], areas[0], areas[2]}
	if g2, ok, _ := r.Resolve(ctx, reordered); !ok || string(g2) != string(g) || store.codeCalls != 1 || store.unionCalls != 1 {
		t.Fatalf("cache miss: codes %d unions %d", store.codeCalls, store.unionCalls)
	}

	// One unknown code fails the whole warning and is not cached.
	unknown := []Area{{Codes: []domain.AreaCode{be}}, {Codes: []domain.AreaCode{{Type: "NUTS3", Value: "XX999"}}}}
	for range 2 {
		if _, ok, err := r.Resolve(ctx, unknown); ok || err != nil {
			t.Fatalf("unknown code resolved: ok %v err %v", ok, err)
		}
	}
	if store.codeCalls != 3 || store.unionCalls != 1 {
		t.Fatalf("codes %d unions %d", store.codeCalls, store.unionCalls)
	}

	// Polygons only: no code lookup.
	if _, ok, _ := r.Resolve(ctx, []Area{{Polygons: []json.RawMessage{poly}}}); !ok || store.codeCalls != 3 {
		t.Fatalf("polygon-only: ok %v codes %d", ok, store.codeCalls)
	}
}

func TestResolverEvictsOverBudget(t *testing.T) {
	store := &fakeAreaStore{codes: map[domain.AreaCode][]string{}}
	r := NewResolver(store)
	r.maxBytes = 200
	ctx := context.Background()
	for i := range 10 {
		p := json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,` + string(rune('1'+i)) + `],[0,0]]]}`)
		if _, ok, _ := r.Resolve(ctx, []Area{{Polygons: []json.RawMessage{p}}}); !ok {
			t.Fatal("not resolved")
		}
	}
	if r.bytes > r.maxBytes || r.order.Len() != len(r.byKey) || r.order.Len() == 0 || r.order.Len() == 10 {
		t.Fatalf("bytes %d of %d, %d entries, %d keys", r.bytes, r.maxBytes, r.order.Len(), len(r.byKey))
	}
}
