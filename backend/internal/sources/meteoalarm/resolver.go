package meteoalarm

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"shield/internal/domain"
)

// AreaStore is what the resolver needs from the warning-area tables
// (store.WarningAreaRepository).
type AreaStore interface {
	Codes(ctx context.Context, geocodes []domain.AreaCode) (map[domain.AreaCode][]string, error)
	Union(ctx context.Context, codes []string, polygons []json.RawMessage) (json.RawMessage, error)
}

// defaultCacheBytes bounds the resolver's memory. A poll repeats almost
// every warning of the last one, so the live set (a few thousand shapes)
// fits and a re-poll costs no queries.
const defaultCacheBytes = 64 << 20

// Resolver draws warnings from the geocodes tables and caches the shapes,
// least recently used out first. Unknown codes are not cached: a reseed
// can add them.
type Resolver struct {
	store    AreaStore
	maxBytes int

	mu    sync.Mutex
	bytes int
	order *list.List // front = most recent; values are *cached
	byKey map[[32]byte]*list.Element
}

type cached struct {
	key  [32]byte
	geom json.RawMessage
}

// NewResolver builds a Resolver over store.
func NewResolver(store AreaStore) *Resolver {
	return &Resolver{store: store, maxBytes: defaultCacheBytes, order: list.New(), byKey: map[[32]byte]*list.Element{}}
}

// Resolve implements AreaResolver: the union of every area's polygons and
// the EMMA_IDs behind its codes. ok is false when any code is unknown.
func (r *Resolver) Resolve(ctx context.Context, areas []Area) (json.RawMessage, bool, error) {
	key := areaKey(areas)
	if g, ok := r.get(key); ok {
		return g, true, nil
	}

	var (
		polygons []json.RawMessage
		geocodes []domain.AreaCode
	)
	for _, a := range areas {
		polygons = append(polygons, a.Polygons...)
		geocodes = append(geocodes, a.Codes...)
	}
	var emma []string
	if len(geocodes) > 0 {
		found, err := r.store.Codes(ctx, geocodes)
		if err != nil {
			return nil, false, err
		}
		for _, g := range geocodes {
			codes, ok := found[g]
			if !ok {
				return nil, false, nil
			}
			emma = append(emma, codes...)
		}
		slices.Sort(emma)
		emma = slices.Compact(emma)
	}
	if len(emma) == 0 && len(polygons) == 0 {
		return nil, false, nil
	}
	geom, err := r.store.Union(ctx, emma, polygons)
	if err != nil {
		return nil, false, err
	}
	r.put(key, geom)
	return geom, true, nil
}

// areaKey identifies a warning's areas regardless of their order.
func areaKey(areas []Area) [32]byte {
	parts := make([]string, 0, len(areas))
	for _, a := range areas {
		var b strings.Builder
		for _, p := range a.Polygons {
			b.WriteString("P")
			b.Write(p)
			b.WriteByte(0)
		}
		for _, c := range a.Codes {
			b.WriteString("C" + c.Type + "=" + c.Value)
			b.WriteByte(0)
		}
		parts = append(parts, b.String())
	}
	slices.Sort(parts)
	return sha256.Sum256([]byte(strings.Join(parts, "\x1e")))
}

func (r *Resolver) get(key [32]byte) (json.RawMessage, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byKey[key]
	if !ok {
		return nil, false
	}
	r.order.MoveToFront(e)
	return e.Value.(*cached).geom, true
}

func (r *Resolver) put(key [32]byte, geom json.RawMessage) {
	if len(geom) > r.maxBytes {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.byKey[key]; ok {
		r.bytes -= len(e.Value.(*cached).geom)
		r.order.Remove(e)
	}
	r.byKey[key] = r.order.PushFront(&cached{key: key, geom: geom})
	r.bytes += len(geom)
	for r.bytes > r.maxBytes {
		last := r.order.Back()
		c := last.Value.(*cached)
		r.order.Remove(last)
		delete(r.byKey, c.key)
		r.bytes -= len(c.geom)
	}
}
