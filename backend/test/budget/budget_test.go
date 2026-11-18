// Package budget checks the S2.7 performance budget against a running
// server: every map-facing request answers within 200 ms at p95 and
// sends at most 300 KB gzipped. It needs real data, so it is skipped
// unless SHIELD_BUDGET_URL is set (make budget); the server must run with
// RATE_LIMIT_RPS=0, or the limiter answers instead of the handlers.
package budget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"
)

const (
	p95Budget  = 200 * time.Millisecond
	sizeBudget = 300 << 10 // gzipped bytes
	requests   = 40        // per endpoint
	workers    = 8
)

type probe struct {
	name, path, accept string
}

func TestBudget(t *testing.T) {
	base := os.Getenv("SHIELD_BUDGET_URL")
	if base == "" {
		t.Skip("SHIELD_BUDGET_URL not set; run `make budget` against a server with data")
	}
	// The raw transport keeps the body gzipped, so sizes are what a client
	// downloads.
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{DisableCompression: true}}

	probes := []probe{
		{"events world (zoom 2)", "/v1/events?zoom=2", "application/json"},
		{"events world GeoJSON (zoom 2)", "/v1/events?zoom=2", "application/geo+json"},
		{"events Europe (zoom 4)", "/v1/events?zoom=4&bbox=-25,34,45,72", "application/json"},
		{"events Iberia GeoJSON (zoom 7)", "/v1/events?zoom=7&bbox=-10,36,4,44", "application/geo+json"},
		{"events, no zoom (list)", "/v1/events", "application/json"},
		{"fires world grid", "/v1/fires", ""},
		{"fires grid (zoom 5, Africa)", "/v1/fires?zoom=5&bbox=-20,-35,52,38", ""},
		{"fires points (zoom 7, Iberia)", "/v1/fires?zoom=7&bbox=-10,36,4,44", ""},
		{"countries", "/v1/countries", ""},
		{"sources", "/v1/sources", ""},
	}
	probes = append(probes, discovered(t, client, base)...)

	failed := false
	t.Logf("%-40s %8s %8s %10s", "endpoint", "p50", "p95", "gzipped")
	for _, p := range probes {
		times, size, err := measure(client, base, p)
		if err != nil {
			t.Errorf("%s: %v", p.name, err)
			failed = true
			continue
		}
		p50, p95 := times[len(times)/2], times[len(times)*95/100]
		mark := ""
		if p95 > p95Budget || size > sizeBudget {
			mark, failed = "  OVER BUDGET", true
		}
		t.Logf("%-40s %8s %8s %8.1fKB%s", p.name, p50.Round(time.Millisecond), p95.Round(time.Millisecond), float64(size)/1024, mark)
	}
	if failed {
		t.Fatalf("budget: p95 ≤ %s, ≤ %d KB gzipped", p95Budget, sizeBudget>>10)
	}
}

type country struct {
	ISO2         string `json:"iso2"`
	ActiveEvents int    `json:"activeEvents"`
}

// discovered adds the heaviest detail views: the most severe events, and
// the busiest country's summary.
func discovered(t *testing.T, client *http.Client, base string) []probe {
	t.Helper()
	var list struct {
		Events []struct{ ID string } `json:"events"`
	}
	getJSON(t, client, base+"/v1/events?limit=5", &list)
	var out []probe
	for _, e := range list.Events {
		out = append(out, probe{"event detail " + e.ID[:8], "/v1/events/" + url.PathEscape(e.ID), ""})
	}
	var countries struct {
		Countries []country `json:"countries"`
	}
	getJSON(t, client, base+"/v1/countries", &countries)
	cs := countries.Countries
	slices.SortFunc(cs, func(a, b country) int { return b.ActiveEvents - a.ActiveEvents })
	if len(cs) > 0 {
		out = append(out, probe{"busiest country summary (" + cs[0].ISO2 + ")", "/v1/countries/" + cs[0].ISO2 + "/summary", ""})
	}
	return out
}

func getJSON(t *testing.T, client *http.Client, u string, v any) {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	resp, err := (&http.Client{Timeout: client.Timeout}).Do(req) // transparent gzip
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: %d", u, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("%s: %v", u, err)
	}
}

// measure issues `requests` full, uncached requests from `workers` at
// once and returns the sorted latencies and the largest gzipped body. One
// unmeasured round first warms the server's connection pool and caches:
// the budget is for a running server, not its first second.
func measure(client *http.Client, base string, p probe) ([]time.Duration, int, error) {
	var warm sync.WaitGroup
	for range workers {
		warm.Go(func() { _, _, _ = once(client, base+p.path, p.accept) })
	}
	warm.Wait()

	var (
		mu    sync.Mutex
		times []time.Duration
		size  int
		first error
		wg    sync.WaitGroup
		next  = make(chan struct{}, requests)
	)
	for range requests {
		next <- struct{}{}
	}
	close(next)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range next {
				d, n, err := once(client, base+p.path, p.accept)
				mu.Lock()
				if err != nil && first == nil {
					first = err
				}
				times = append(times, d)
				size = max(size, n)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	return times, size, first
}

func once(client *http.Client, u, accept string) (time.Duration, int, error) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	d := time.Since(start)
	if err != nil {
		return d, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return d, 0, fmt.Errorf("%s: status %d", u, resp.StatusCode)
	}
	return d, int(n), nil
}
