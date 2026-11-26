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
	d := &server.Deps{Config: config.Base{}, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	f := New(r)
	f.now = func() time.Time { return now }
	srv, err := server.New(d, []server.Feature{f})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	return w
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
	for _, bad := range []string{
		"since=2026-11-18T11:59:59Z", // more than 7 days ago
		"since=2026-11-26T12:00:01Z", // in the future
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
