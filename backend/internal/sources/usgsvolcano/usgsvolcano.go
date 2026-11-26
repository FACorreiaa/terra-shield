// Package usgsvolcano ingests the alert levels of US volcanoes from the
// USGS Volcano Hazards Program (ADR 35).
//
// Feeds, both from the Volcano Science Center API, every 5 minutes:
//   - /vsc/api/volcanoApi/geojson: every US volcano (161 on 2026-11-26)
//     with its ground alert level (NORMAL, ADVISORY, WATCH, WARNING) and
//     aviation color code (GREEN, YELLOW, ORANGE, RED), or UNASSIGNED
//     where an observatory sets none. It is the state of record.
//   - /vsc/api/volcanoApi/elevated: the volcanoes above normal, adding
//     when the current level began and the levels before it.
//
// One event per volcano while it is above normal, keyed by its GVP volcano
// number (the join key with EONET's GVP volcanoes, ADR 5), or by USGS's own
// volcano code for the few GVP does not number (Folsoms Bluff). It ends when
// the list shows it back at NORMAL or UNASSIGNED, at the notice that said
// so: the full list names every volcano's state, so an absence is never
// read as an ending.
//
// License: public domain (USGS-authored); USGS asks to be credited.
package usgsvolcano

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/metrics"
	"shield/internal/severity"
)

const (
	// ID is the source key.
	ID = "usgs-volcano"
	// StatusURL lists every US volcano and its alert levels.
	StatusURL = "https://volcanoes.usgs.gov/vsc/api/volcanoApi/geojson"
	// ElevatedURL lists the volcanoes above normal, with their history.
	ElevatedURL = "https://volcanoes.usgs.gov/vsc/api/volcanoApi/elevated"
	// PollInterval: notices are issued as activity changes, daily for an
	// erupting volcano; neither feed sends validators.
	PollInterval = 5 * time.Minute
)

// Coverage is where USGS observatories set alert levels: the states, the
// Northern Mariana Islands and American Samoa.
var Coverage = []string{"US", "MP", "AS"}

// Store is what the adapter needs.
type Store interface {
	ingest.EventStore
	ActiveIDs(ctx context.Context, sourceID string) ([]string, error)
	Resolve(ctx context.Context, sourceID string, sourceEventIDs []string, at time.Time) (int, error)
}

// Source is the adapter.
type Source struct {
	statusURL   string
	elevatedURL string
	client      *fetch.Client
	store       Store
	logger      *slog.Logger
	now         func() time.Time
}

// New builds the adapter on the production feeds.
func New(client *fetch.Client, store Store, logger *slog.Logger) *Source {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Source{
		statusURL: StatusURL, elevatedURL: ElevatedURL,
		client: client, store: store, logger: logger, now: time.Now,
	}
}

// Info implements ingest.Source.
func (*Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:               ID,
		Name:             "USGS Volcano Hazards Program",
		URL:              "https://www.usgs.gov/programs/VHP",
		License:          "Public domain (USGS)",
		Attribution:      "U.S. Geological Survey Volcano Hazards Program",
		TermsURL:         "https://www.usgs.gov/information-policies-and-instructions/copyrights-and-credits",
		Scope:            "Official alert levels of US volcanoes (Alaska, Hawaii, the West, the Northern Mariana Islands, American Samoa)",
		Cadence:          "as observatories issue notices; daily for an erupting volcano",
		Coverage:         Coverage,
		OfficialWarnings: true,
		PollInterval:     PollInterval,
		Enabled:          true,
	}
}

// Poll implements ingest.Source. Both feeds are read before anything is
// written.
func (s *Source) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	status, err := s.client.Get(ctx, s.statusURL, domain.Validators{})
	if err != nil {
		return ingest.Result{}, err
	}
	elevated, err := s.client.Get(ctx, s.elevatedURL, domain.Validators{})
	if err != nil {
		return ingest.Result{}, err
	}
	f, err := Parse(status.Body, elevated.Body)
	if err != nil {
		return ingest.Result{}, err
	}
	if f.Unreadable > 0 {
		metrics.SourceSkipped.WithLabelValues(ID, "unreadable").Add(float64(f.Unreadable))
		s.logger.Warn("usgs volcanoes unreadable", "count", f.Unreadable)
	}
	n, err := s.store.Upsert(ctx, ID, f.Events)
	if err != nil {
		return ingest.Result{}, err
	}
	active, err := s.store.ActiveIDs(ctx, ID)
	if err != nil {
		return ingest.Result{}, err
	}
	r := 0
	for _, id := range active {
		at, quiet := f.Quiet[id]
		if !quiet {
			continue // still elevated, or not listed: never read as an ending
		}
		if at.IsZero() || at.After(s.now()) {
			at = s.now()
		}
		k, err := s.store.Resolve(ctx, ID, []string{id}, at)
		if err != nil {
			return ingest.Result{}, err
		}
		r += k
	}
	return ingest.Result{Upserted: n + r}, nil
}

// Feed is one reading of both feeds.
type Feed struct {
	Events []domain.Event
	// Quiet maps each volcano listed at NORMAL or UNASSIGNED to the time
	// of its latest notice (zero when it has none).
	Quiet map[string]time.Time
	// Unreadable counts volcanoes above normal without an id, a position
	// or any time, which are not stored.
	Unreadable int
}

type statusFeed struct {
	Features []struct {
		Geometry struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties statusProps `json:"properties"`
	} `json:"features"`
}

type statusProps struct {
	Name        string `json:"volcanoName"`
	Vnum        string `json:"vnum"`
	Code        string `json:"volcanoCd"`
	VolcanoURL  string `json:"volcanoUrl"`
	Observatory string `json:"obs"`
	Region      string `json:"region"`
	NoticeID    string `json:"noticeId"`
	Synopsis    string `json:"noticeSynopsis"`
	NoticeURL   string `json:"noticeUrl"`
	AlertLevel  string `json:"alertLevel"`
	ColorCode   string `json:"colorCode"`
	AlertDate   string `json:"alertDate"`
	ColorDate   string `json:"colorDate"`
	Threat      string `json:"nvewsThreat"`
}

type elevatedRecord struct {
	Vnum           string `json:"vnum"`
	AlertLevelPrev string `json:"alertLevelPrev"`
	ColorCodePrev  string `json:"colorCodePrev"`
	CodeChangeDate string `json:"codeChangeDate"`
}

// timeLayout is how both feeds write UTC times.
const timeLayout = "2006-01-02 15:04:05"

// Parse reads the status list and the elevated list. The status list is
// the state of record; the elevated list only adds history, and a volcano
// missing from it is stored without a start.
func Parse(status, elevated []byte) (Feed, error) {
	var st statusFeed
	if err := json.Unmarshal(status, &st); err != nil {
		return Feed{}, fmt.Errorf("usgs-volcano: decode status: %w", err)
	}
	if len(st.Features) == 0 {
		return Feed{}, fmt.Errorf("usgs-volcano: status list is empty")
	}
	var el []elevatedRecord
	if err := json.Unmarshal(elevated, &el); err != nil {
		return Feed{}, fmt.Errorf("usgs-volcano: decode elevated: %w", err)
	}
	history := make(map[string]elevatedRecord, len(el))
	for _, r := range el {
		if r.Vnum != "" {
			history[r.Vnum] = r
		}
	}

	out := Feed{Quiet: map[string]time.Time{}}
	for i := range st.Features {
		ft := &st.Features[i]
		p := &ft.Properties
		latest := latestOf(p.AlertDate, p.ColorDate)
		id := eventID(p)
		level, elevated := Classify(p.AlertLevel, p.ColorCode)
		if !elevated {
			if id != "" {
				out.Quiet[id] = latest
			}
			continue
		}
		if id == "" || ft.Geometry.Type != "Point" || len(ft.Geometry.Coordinates) < 2 {
			out.Unreadable++
			continue
		}
		geom, err := json.Marshal(map[string]any{"type": "Point", "coordinates": ft.Geometry.Coordinates[:2]})
		if err != nil {
			return Feed{}, err
		}
		attrs := map[string]any{
			"alertLevel":  p.AlertLevel,
			"colorCode":   p.ColorCode,
			"observatory": p.Observatory,
		}
		for k, v := range map[string]string{
			"vnum": p.Vnum, "volcanoCode": p.Code, "volcanoUrl": p.VolcanoURL, "region": p.Region,
			"noticeId": p.NoticeID, "threat": p.Threat,
		} {
			if v != "" {
				attrs[k] = v
			}
		}
		var started *time.Time
		if h, ok := history[p.Vnum]; ok {
			if h.AlertLevelPrev != "" {
				attrs["alertLevelPrev"] = h.AlertLevelPrev
			}
			if h.ColorCodePrev != "" {
				attrs["colorCodePrev"] = h.ColorCodePrev
			}
			if t, err := time.Parse(timeLayout, h.CodeChangeDate); err == nil {
				started = &t
			}
		}
		updated := latest
		if updated.IsZero() && started != nil {
			updated = *started
		}
		if updated.IsZero() {
			out.Unreadable++ // no time at all: nothing honest to show
			continue
		}
		url := p.NoticeURL
		if url == "" {
			url = p.VolcanoURL
		}
		title := p.Name
		if p.Region != "" {
			title += ", " + p.Region
		}
		out.Events = append(out.Events, domain.Event{
			SourceEventID:  id,
			Type:           domain.TypeVolcano,
			Kind:           domain.KindWarning,
			Status:         domain.StatusActive,
			Severity:       level.String(),
			SourceSeverity: sourceSeverity(p.AlertLevel, p.ColorCode),
			Title:          title,
			Summary:        strings.TrimSpace(p.Synopsis),
			Geometry:       geom,
			StartedAt:      started,
			ObservedAt:     updated,
			UpdatedAt:      updated,
			URL:            url,
			Attrs:          attrs,
		})
	}
	return out, nil
}

// eventID is the GVP volcano number, else USGS's volcano code.
func eventID(p *statusProps) string {
	if p.Vnum != "" {
		return p.Vnum
	}
	if p.Code != "" {
		return "usgs:" + p.Code
	}
	return ""
}

// latestOf is the later of the alert-level and color-code times.
func latestOf(times ...string) time.Time {
	var out time.Time
	for _, s := range times {
		if t, err := time.Parse(timeLayout, s); err == nil && t.After(out) {
			out = t
		}
	}
	return out
}

var groundLevels = map[string]severity.Level{
	"ADVISORY": severity.Moderate,
	"WATCH":    severity.Severe,
	"WARNING":  severity.Extreme,
}

var aviationColors = map[string]severity.Level{
	"YELLOW": severity.Moderate,
	"ORANGE": severity.Severe,
	"RED":    severity.Extreme,
}

// Classify derives severity from the ground alert level, which is about
// people on the ground (docs/severity.md). Only a volcano with no ground
// level (UNASSIGNED: a seamount, an unmonitored volcano) is rated by its
// aviation color code. It reports false for a volcano at normal.
func Classify(alertLevel, colorCode string) (severity.Level, bool) {
	if l, ok := groundLevels[strings.ToUpper(alertLevel)]; ok {
		return l, true
	}
	if strings.EqualFold(alertLevel, "NORMAL") {
		return 0, false
	}
	l, ok := aviationColors[strings.ToUpper(colorCode)]
	return l, ok
}

// sourceSeverity is both of USGS's levels, as USGS names them: "Advisory
// · aviation Yellow".
func sourceSeverity(alertLevel, colorCode string) string {
	var parts []string
	if a := strings.ToUpper(alertLevel); a != "" && a != "UNASSIGNED" {
		parts = append(parts, title(a))
	}
	if c := strings.ToUpper(colorCode); c != "" && c != "UNASSIGNED" {
		parts = append(parts, "aviation "+title(c))
	}
	return strings.Join(parts, " · ")
}

func title(s string) string {
	return s[:1] + strings.ToLower(s[1:])
}
