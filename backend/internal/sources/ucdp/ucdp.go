// Package ucdp ingests armed-conflict events from the Uppsala Conflict
// Data Program's Candidate Events Dataset (ADR 27).
//
// UCDP publishes one global CSV per month, around the 20th of the next
// month, at a URL fixed by its naming convention (year.0.month, e.g.
// GEDEvent_v26_0_9.csv for September 2026). Each event is an incident of
// organized lethal violence at a place and date, geocoded with a stated
// precision and carrying fatality estimates. The data are research
// estimates, not live reports: "candidate" events may be revised or
// dropped in UCDP's yearly dataset, and the newest month is one to two
// months old.
//
// An event is shown for Window after its end date (attrs.expiresAt), then
// resolved by the expiry sweep. Each poll reads every monthly file that
// can hold an event inside that window, so the database holds the window
// and a newer file never removes anything (UCDP retracts nothing between
// releases).
//
// License CC BY 4.0, citation required. The news-source columns (articles,
// headlines, quotes) are never stored: they are third-party text and name
// people.
package ucdp

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/severity"
)

const (
	// ID is the source key in the sources table.
	ID = "ucdp"
	// BaseURL is where the monthly Candidate files live.
	BaseURL = "https://ucdp.uu.se/downloads/candidateged/"
	// PollInterval: a new month appears once a month, on a day UCDP does
	// not fix; four polls a day find it within hours.
	PollInterval = 6 * time.Hour
	// Window is how long after its end date an event stays on the map.
	Window = 90 * 24 * time.Hour
	// batch keeps each commit under a stream client's queue (#29).
	batch = 200
)

// Store is what the adapter writes.
type Store interface {
	ingest.EventStore
	ResolveExpired(ctx context.Context, sourceID string, now time.Time) (int, error)
}

// Source is the UCDP adapter.
type Source struct {
	base   string
	client *fetch.Client
	store  Store
	now    func() time.Time
}

// New builds the adapter on UCDP's download site.
func New(client *fetch.Client, store Store) *Source {
	return &Source{base: BaseURL, client: client, store: store, now: time.Now}
}

// Info implements ingest.Source.
func (*Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "UCDP Candidate Events (Uppsala Conflict Data Program)",
		URL:          "https://ucdp.uu.se/downloads/",
		License:      "CC BY 4.0",
		Attribution:  "Uppsala Conflict Data Program, UCDP Candidate Events Dataset (Hegre et al. 2020)",
		PollInterval: PollInterval,
		Enabled:      true,
		NonHazard:    true,
	}
}

// PollTimeout implements ingest.SlowSource: a first poll reads three
// files (about 1.5 MB each, a second apart) and stores a few thousand
// events in batches.
func (*Source) PollTimeout() time.Duration { return 5 * time.Minute }

var _ ingest.SlowSource = (*Source)(nil)

// FileName is the monthly release holding events of month m.
func FileName(year int, m time.Month) string {
	return fmt.Sprintf("GEDEvent_v%02d_0_%d.csv", year%100, int(m))
}

// months are the releases that can hold an event inside the window at
// now, oldest first: from the month Window ago to last month (this
// month's file cannot exist yet).
func months(now time.Time) []time.Time {
	first := monthOf(now.Add(-Window))
	last := monthOf(now).AddDate(0, -1, 0)
	var out []time.Time
	for m := first; !m.After(last); m = m.AddDate(0, 1, 0) {
		out = append(out, m)
	}
	return out
}

func monthOf(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Poll implements ingest.Source.
func (s *Source) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	now := s.now().UTC()
	// The sweep first, so events still leave the map while UCDP is down.
	expired, err := s.store.ResolveExpired(ctx, ID, now)
	if err != nil {
		return ingest.Result{}, err
	}

	byID := map[string]domain.Event{}
	var newest time.Time
	found := 0
	for _, m := range months(now) {
		// Every file in full, each poll: the window is three files, and a
		// re-sent unchanged event costs no write.
		resp, err := s.client.Get(ctx, s.base+FileName(m.Year(), m.Month()), domain.Validators{})
		var se *fetch.StatusError
		if errors.As(err, &se) && se.Status == http.StatusNotFound {
			continue // not released yet
		}
		if err != nil {
			return ingest.Result{}, err
		}
		events, err := Parse(resp.Body)
		if err != nil {
			return ingest.Result{}, fmt.Errorf("%s: %w", FileName(m.Year(), m.Month()), err)
		}
		found++
		if t, err := http.ParseTime(resp.Validators.LastModified); err == nil && t.After(newest) {
			newest = t
		}
		for i := range events {
			byID[events[i].SourceEventID] = events[i] // a newer month wins
		}
	}
	if found == 0 {
		return ingest.Result{}, errors.New("ucdp: no monthly release found; has the naming changed?")
	}

	live := make([]domain.Event, 0, len(byID))
	for id := range byID {
		if byID[id].ObservedAt.Add(Window).After(now) {
			live = append(live, byID[id])
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].SourceEventID < live[j].SourceEventID })
	written := expired
	for from := 0; from < len(live); from += batch {
		n, err := s.store.Upsert(ctx, ID, live[from:min(from+batch, len(live))])
		if err != nil {
			return ingest.Result{}, err
		}
		written += n
	}
	return ingest.Result{Upserted: written, SourceUpdatedAt: newest}, nil
}

// columns are the fields read; a release missing one fails whole.
var columns = []string{
	"id", "code_status", "type_of_violence", "conflict_name", "side_a", "side_b", "number_of_sources",
	"where_prec", "where_description", "latitude", "longitude", "country_id", "date_prec", "date_start",
	"date_end", "deaths_a", "deaths_b", "deaths_civilians", "deaths_unknown", "best", "high", "low",
}

var (
	violence = map[string]string{"1": "state_based", "2": "non_state", "3": "one_sided"}
	// Location precision 6 (only the country) and 7 (international
	// waters or airspace) give no place to draw, so those events are
	// skipped.
	wherePrec = map[string]string{"1": "exact", "2": "within_25km", "3": "admin2", "4": "admin1", "5": "area"}
	datePrec  = map[string]string{"1": "day", "2": "days_2_6", "3": "week", "4": "month", "5": "months"}
	// UCDP writes XXX and a country number for an actor it cannot name.
	unnamed = regexp.MustCompile(`\bXXX\d+\b`)
	idRe    = regexp.MustCompile(`^[0-9]{1,12}$`)
)

// Parse reads one monthly release. Any malformed record fails the file:
// a half-read month would show a war as quieter than it was.
func Parse(body []byte) ([]domain.Event, error) {
	r := csv.NewReader(bytes.NewReader(body))
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("ucdp: header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.TrimPrefix(h, "\ufeff")] = i
	}
	for _, c := range columns {
		if _, ok := col[c]; !ok {
			return nil, fmt.Errorf("ucdp: no %s column", c)
		}
	}
	var out []domain.Event
	for line := 2; ; line++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("ucdp: %w", err)
		}
		f := func(name string) string { return strings.TrimSpace(rec[col[name]]) }
		e, ok, err := event(f)
		if err != nil {
			return nil, fmt.Errorf("ucdp: record %d (id %q): %w", line, f("id"), err)
		}
		if ok {
			out = append(out, e)
		}
	}
}

// event maps one record; ok is false for an event with no place to draw.
func event(f func(string) string) (e domain.Event, ok bool, err error) {
	id := f("id")
	if !idRe.MatchString(id) {
		return e, false, errors.New("bad id")
	}
	vt, okV := violence[f("type_of_violence")]
	dp, okD := datePrec[f("date_prec")]
	if !okV || !okD {
		return e, false, fmt.Errorf("bad type_of_violence %q or date_prec %q", f("type_of_violence"), f("date_prec"))
	}
	wp, mappable := wherePrec[f("where_prec")]
	if !mappable {
		if p := f("where_prec"); p != "6" && p != "7" {
			return e, false, fmt.Errorf("bad where_prec %q", p)
		}
		return e, false, nil
	}
	var n [8]int
	for i, name := range []string{"best", "low", "high", "deaths_a", "deaths_b", "deaths_civilians", "deaths_unknown", "number_of_sources"} {
		if n[i], err = strconv.Atoi(f(name)); err != nil || n[i] < 0 {
			return e, false, fmt.Errorf("bad %s %q", name, f(name))
		}
	}
	best, low, high := n[0], n[1], n[2]
	lat, err1 := strconv.ParseFloat(f("latitude"), 64)
	lon, err2 := strconv.ParseFloat(f("longitude"), 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return e, false, fmt.Errorf("bad coordinates %q,%q", f("latitude"), f("longitude"))
	}
	start, err1 := day(f("date_start"))
	end, err2 := day(f("date_end"))
	if err1 != nil || err2 != nil || end.Before(start) {
		return e, false, fmt.Errorf("bad dates %q..%q", f("date_start"), f("date_end"))
	}
	countryID, err := strconv.Atoi(f("country_id"))
	if err != nil {
		return e, false, fmt.Errorf("bad country_id %q", f("country_id"))
	}

	sideA, sideB := name(f("side_a")), name(f("side_b"))
	status := f("code_status")
	attrs := map[string]any{
		"violenceType":      vt,
		"conflictName":      name(f("conflict_name")),
		"sideA":             sideA,
		"sideB":             sideB,
		"deathsBest":        best,
		"deathsLow":         low,
		"deathsHigh":        high,
		"deathsSideA":       n[3],
		"deathsSideB":       n[4],
		"deathsCivilians":   n[5],
		"deathsUnknown":     n[6],
		"sourceCount":       n[7],
		"locationPrecision": wp,
		"datePrecision":     dp,
		"codeStatus":        status,
		"expiresAt":         end.Add(Window).Format(time.RFC3339),
	}
	// Fatalities still being checked, a doubtful source, or an open
	// question make the level, which rests on the count, provisional; so
	// does a date known only to the month or wider, which is a summary of
	// many incidents (a cartel war over nine months) rather than one.
	if s := strings.ToLower(status); strings.Contains(s, "deaths") || strings.Contains(s, "vague") || s == "check" ||
		dp == "month" || dp == "months" {
		attrs["severityProvisional"] = true
	}
	return domain.Event{
		SourceEventID:  id,
		Type:           domain.TypeConflict,
		Kind:           domain.KindObserved,
		Status:         domain.StatusActive,
		Severity:       severity.ConflictFatalities(best).String(),
		SourceSeverity: deaths(best, low, high),
		Title:          sideA + " – " + sideB,
		Summary:        f("where_description"),
		Geometry:       domain.PointGeometry(lon, lat),
		StartedAt:      &start,
		// The event's own last day: UCDP gives no edit time, and a release
		// date would read as news.
		ObservedAt: end,
		UpdatedAt:  end,
		URL:        "https://ucdp.uu.se/country/" + strconv.Itoa(countryID),
		Attrs:      attrs,
	}, true, nil
}

// day reads UCDP's dates: "2026-09-21 00:00:00.000", or a bare date.
func day(s string) (time.Time, error) {
	if len(s) >= 10 {
		return time.Parse(time.DateOnly, s[:10])
	}
	return time.Time{}, errors.New("short date")
}

func name(s string) string {
	return unnamed.ReplaceAllString(s, "Unidentified group")
}

// deaths is the source's own figure: "28 deaths (3–31)", "1 death".
func deaths(best, low, high int) string {
	unit := "deaths"
	if best == 1 {
		unit = "death"
	}
	if low == best && high == best {
		return fmt.Sprintf("%d %s", best, unit)
	}
	return fmt.Sprintf("%d %s (%d–%d)", best, unit, low, high)
}
