package changes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"shield/internal/config"
	"shield/internal/domain"
	"shield/internal/server"
)

var now = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

type fakeRepo struct {
	rows []domain.Change
	q    domain.ChangeQuery
}

func (f *fakeRepo) List(_ context.Context, q domain.ChangeQuery) ([]domain.Change, error) {
	f.q = q
	return f.rows, nil
}

func get(t *testing.T, r Repository, target string) *httptest.ResponseRecorder {
	t.Helper()
	return getAt(t, r, target, now, "")
}

// getAt asks at a given time, with an If-None-Match when etag is set.
func getAt(t *testing.T, r Repository, target string, at time.Time, etag string) *httptest.ResponseRecorder {
	t.Helper()
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	f := New(r)
	f.now = func() time.Time { return at }
	srv, err := server.New(d, []server.Feature{f})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	return w
}

// The same changes asked a few seconds later keep their ETag, so a
// client polling the default window gets a 304.
func TestETag(t *testing.T) {
	repo := &fakeRepo{rows: []domain.Change{{EventID: "a", Change: domain.ChangeNew, Type: domain.TypeFlood, Severity: "minor", At: now}}}
	first := get(t, repo, "/v1/changes")
	tag := first.Header().Get("ETag")
	if first.Code != http.StatusOK || tag == "" {
		t.Fatalf("first: %d, etag %q", first.Code, tag)
	}
	if w := getAt(t, repo, "/v1/changes", now.Add(5*time.Second), tag); w.Code != http.StatusNotModified {
		t.Fatalf("same changes later: %d, want 304", w.Code)
	}
	repo.rows[0].Severity = "severe"
	if w := getAt(t, repo, "/v1/changes", now.Add(5*time.Second), tag); w.Code != http.StatusOK {
		t.Fatalf("changed: %d, want 200", w.Code)
	}
}

func TestParams(t *testing.T) {
	repo := &fakeRepo{}
	if w := get(t, repo, "/v1/changes"); w.Code != http.StatusOK {
		t.Fatalf("default: %d %s", w.Code, w.Body)
	}
	if !repo.q.Since.Equal(now.Add(-24*time.Hour)) || len(repo.q.Types) == 0 || slices.Contains(repo.q.Types, domain.TypeConflict) {
		t.Fatalf("default query %+v", repo.q)
	}
	if w := get(t, repo, "/v1/changes?since=2026-11-20T12:00:00Z&country=pt&severity=severe,extreme&type=flood"); w.Code != http.StatusOK {
		t.Fatalf("filters: %d %s", w.Code, w.Body)
	}
	if repo.q.Country != "PT" || !slices.Equal(repo.q.Severities, []string{"severe", "extreme"}) ||
		!slices.Equal(repo.q.Types, []domain.EventType{domain.TypeFlood}) || !repo.q.Since.Equal(now.Add(-6*24*time.Hour)) {
		t.Fatalf("filtered query %+v", repo.q)
	}
	// A client clock a little off is clamped to the bound.
	for target, want := range map[string]time.Time{
		"since=2026-11-19T11:59:30Z": now.Add(-MaxWindow),
		"since=2026-11-26T12:00:30Z": now,
	} {
		if w := get(t, repo, "/v1/changes?"+target); w.Code != http.StatusOK || !repo.q.Since.Equal(want) {
			t.Errorf("%s: %d, since %v, want %v", target, w.Code, repo.q.Since, want)
		}
	}
	for _, bad := range []string{
		"since=2026-11-19T11:58:59Z", // more than 7 days and a minute ago
		"since=2026-11-26T12:01:01Z", // more than a minute ahead
		"since=yesterday",
		"limit=0", "limit=501", "limit=x",
		"severity=bad", "country=PRT", "type=nope",
	} {
		if w := get(t, repo, "/v1/changes?"+bad); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", bad, w.Code)
		}
	}
}

// Counts cover every change; the list is cut at limit, most severe first,
// newest first within a severity.
func TestAnswer(t *testing.T) {
	var rows []domain.Change
	for i := range 6 {
		rows = append(rows, domain.Change{
			EventID: fmt.Sprintf("e%d", i), Type: domain.TypeFlood, Severity: []string{"minor", "severe"}[i%2],
			Change: domain.ChangeKinds()[i%4], At: now.Add(-time.Duration(i) * time.Hour),
		})
	}
	rows = append(rows, domain.Change{EventID: "q", Type: domain.TypeEarthquake, Severity: "extreme", Change: domain.ChangeNew, At: now.Add(-20 * time.Hour)})
	w := get(t, &fakeRepo{rows: rows}, "/v1/changes?limit=3")
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var a Answer
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.Counts["new"] != 3 || a.Counts["escalated"] != 2 || a.Counts["downgraded"] != 1 || a.Counts["ended"] != 1 {
		t.Fatalf("counts %v", a.Counts)
	}
	if !a.Truncated || len(a.Changes) != 3 {
		t.Fatalf("truncated %v, %d listed", a.Truncated, len(a.Changes))
	}
	var order []string
	for _, c := range a.Changes {
		order = append(order, c.EventID)
	}
	if !slices.Equal(order, []string{"q", "e1", "e3"}) {
		t.Fatalf("order %v, want extreme first, then severe newest first", order)
	}
	if len(a.ByType) != 2 || a.ByType[0].Type != "earthquake" || a.ByType[1].Counts["new"] != 2 {
		t.Fatalf("byType %+v", a.ByType)
	}
	if a.Changes[0].Countries == nil {
		t.Fatal("countries should be [] not null")
	}
	// An empty window still answers every count.
	w = get(t, &fakeRepo{}, "/v1/changes")
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil || len(a.Counts) != 4 || a.Changes == nil || a.ByType == nil {
		t.Fatalf("empty: %s %v", w.Body, err)
	}
}
