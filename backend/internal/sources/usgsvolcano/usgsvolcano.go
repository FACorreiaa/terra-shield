// Package usgsvolcano ingests the alert levels of US volcanoes from the
// USGS Volcano Hazards Program (ADR 35).
//
// Feeds, both from the Volcano Science Center API, every 5 minutes:
//   - /vsc/api/volcanoApi/geojson: every US volcano (161 on 2026-11-26)
//     with its ground alert level (NORMAL, ADVISORY, WATCH, WARNING) and
//     aviation color code (GREEN, YELLOW, ORANGE, RED), or UNASSIGNED
//     where an observatory sets none. It is the state of record.
//   - /vsc/api/volcanoApi/elevated: the volcanoes above normal, adding
//     when the current level began and the levels before it. It only adds
//     history: if it fails or disagrees, the poll goes on without it.
//
// One event per volcano while it is above normal, keyed by USGS's volcano
// code (every volcano has one; GVP does not number them all), with the GVP
// number in attrs as the join key with EONET's volcanoes (ADR 5). It ends
// only when the list names it at NORMAL, or UNASSIGNED with no aviation
// color: a level missing, null or unknown is unreadable, and a volcano
// missing from the list is never read as ended.
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
	// noticeTrust is how old a "back to normal" notice may be and still
	// date the ending; an older one (seen after an outage, or a list that
	// reverted) ends the event at the poll instead.
	noticeTrust = 7 * 24 * time.Hour
)

// Coverage is where USGS observatories set alert levels: the states, the
// Northern Mariana Islands and American Samoa.
var Coverage = []string{"US", "MP", "AS"}

// Store is what the adapter needs.
type Store interface {
	ingest.EventStore
	ActiveStarts(ctx context.Context, sourceID string) (map[string]time.Time, error)
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

// Poll implements ingest.Source.
func (s *Source) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	status, err := s.client.Get(ctx, s.statusURL, domain.Validators{})
	if err != nil {
		return ingest.Result{}, err
	}
	var history []byte
	if resp, err := s.client.Get(ctx, s.elevatedURL, domain.Validators{}); err != nil {
		// An escalation must not wait on the history feed.
		metrics.SourceSkipped.WithLabelValues(ID, "history_unavailable").Inc()
		s.logger.Warn("usgs volcano history unavailable; stored starts kept", "err", err)
	} else {
		history = resp.Body
	}
	f, err := Parse(status.Body, history)
	if err != nil {
		return ingest.Result{}, err
	}
	if f.Unreadable > 0 {
		metrics.SourceSkipped.WithLabelValues(ID, "unreadable").Add(float64(f.Unreadable))
		s.logger.Warn("usgs volcanoes unreadable; neither stored nor ended", "count", f.Unreadable)
	}
	stored, err := s.store.ActiveStarts(ctx, ID)
	if err != nil {
		return ingest.Result{}, err
	}
	now := s.now()
	for i := range f.Events {
		e := &f.Events[i]
		// Without matching history, keep the start already stored rather
		// than clear it for a poll.
		if start, ok := stored[e.SourceEventID]; ok && e.StartedAt == nil && !start.IsZero() {
			e.StartedAt = &start
		}
		if e.UpdatedAt.IsZero() { // no notice time at all: seen now
			e.ObservedAt, e.UpdatedAt = now, now
		}
	}
	n, err := s.store.Upsert(ctx, ID, f.Events)
	if err != nil {
		return ingest.Result{}, err
	}
	r := 0
	var unlisted []string
	for id, start := range stored {
		if !f.Listed[id] {
			unlisted = append(unlisted, id)
			continue
		}
		at, quiet := f.Quiet[id]
		if !quiet {
			continue
		}
		if at.IsZero() || at.After(now) || at.Before(start) || now.Sub(at) > noticeTrust {
			at = now
		}
		k, err := s.store.Resolve(ctx, ID, []string{id}, at)
		if err != nil {
			return ingest.Result{}, err
		}
		r += k
	}
	if len(unlisted) > 0 {
		metrics.SourceSkipped.WithLabelValues(ID, "unlisted").Add(float64(len(unlisted)))
		s.logger.Warn("usgs volcanoes stored but no longer listed; kept", "ids", unlisted)
	}
	return ingest.Result{Upserted: n + r}, nil
}

// Feed is one reading of both feeds.
type Feed struct {
	Events []domain.Event
	// Quiet maps each volcano the list names at normal to the time of its
	// latest notice (zero when it has none).
	Quiet map[string]time.Time
	// Listed is every volcano id the status list names, readable or not.
	Listed map[string]bool
	// Unreadable counts volcanoes with a level that is missing or unknown,
	// or elevated without a position, which are neither stored nor ended.
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
	Code           string `json:"volcanoCd"`
	AlertLevel     string `json:"alertLevel"`
	ColorCode      string `json:"colorCode"`
	AlertLevelPrev string `json:"alertLevelPrev"`
	ColorCodePrev  string `json:"colorCodePrev"`
	CodeChangeDate string `json:"codeChangeDate"`
}

// timeLayout is how both feeds write UTC times.
const timeLayout = "2006-01-02 15:04:05"

// Parse reads the status list and, when given, the elevated list. The
// status list is the state of record; elevated history is applied only
// where its levels match the status list's.
func Parse(status, elevatedList []byte) (Feed, error) {
	var st statusFeed
	if err := json.Unmarshal(status, &st); err != nil {
		return Feed{}, fmt.Errorf("usgs-volcano: decode status: %w", err)
	}
	if len(st.Features) == 0 {
		return Feed{}, fmt.Errorf("usgs-volcano: status list is empty")
	}
	history := map[string]elevatedRecord{}
	if len(elevatedList) > 0 {
		var el []elevatedRecord
		if err := json.Unmarshal(elevatedList, &el); err != nil {
			return Feed{}, fmt.Errorf("usgs-volcano: decode elevated: %w", err)
		}
		for _, r := range el {
			if r.Code != "" {
				history[r.Code] = r
			}
		}
	}

	out := Feed{Quiet: map[string]time.Time{}, Listed: map[string]bool{}}
	stored := map[string]bool{}
	for i := range st.Features {
		ft := &st.Features[i]
		p := &ft.Properties
		id := p.Code
		if id == "" {
			out.Unreadable++
			continue
		}
		out.Listed[id] = true
		level, state := Classify(p.AlertLevel, p.ColorCode)
		switch state {
		case Unknown:
			out.Unreadable++
			continue
		case Quiet:
			if !stored[id] { // an id listed twice, once elevated, stays elevated
				out.Quiet[id] = latestOf(p.AlertDate, p.ColorDate)
			}
			continue
		}
		if ft.Geometry.Type != "Point" || len(ft.Geometry.Coordinates) < 2 {
			out.Unreadable++
			continue
		}
		geom, err := json.Marshal(map[string]any{"type": "Point", "coordinates": ft.Geometry.Coordinates[:2]})
		if err != nil {
			return Feed{}, err
		}
		attrs := map[string]any{
			"alertLevel":  norm(p.AlertLevel),
			"colorCode":   norm(p.ColorCode),
			"volcanoCode": p.Code,
		}
		for k, v := range map[string]string{
			"vnum": p.Vnum, "observatory": p.Observatory, "volcanoUrl": p.VolcanoURL,
			"region": p.Region, "noticeId": p.NoticeID, "threat": p.Threat,
		} {
			if v != "" {
				attrs[k] = v
			}
		}
		var started *time.Time
		if h, ok := history[id]; ok && norm(h.AlertLevel) == norm(p.AlertLevel) && norm(h.ColorCode) == norm(p.ColorCode) {
			if h.AlertLevelPrev != "" {
				attrs["alertLevelPrev"] = norm(h.AlertLevelPrev)
			}
			if h.ColorCodePrev != "" {
				attrs["colorCodePrev"] = norm(h.ColorCodePrev)
			}
			if t, err := time.Parse(timeLayout, h.CodeChangeDate); err == nil {
				started = &t
			}
		}
		updated := latestOf(p.AlertDate, p.ColorDate)
		if updated.IsZero() && started != nil {
			updated = *started
		}
		url := p.NoticeURL
		if url == "" {
			url = p.VolcanoURL
		}
		title := p.Name
		if p.Region != "" {
			title += ", " + p.Region
		}
		stored[id] = true
		delete(out.Quiet, id)
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
			ObservedAt:     updated, // zero when no time at all: Poll sets the poll's
			UpdatedAt:      updated,
			URL:            url,
			Attrs:          attrs,
		})
	}
	return out, nil
}

// latestOf is the later of the alert-level and color-code times.
func latestOf(times ...string) time.Time {
	var out time.Time
	for _, s := range times {
		if t, err := time.Parse(timeLayout, strings.TrimSpace(s)); err == nil && t.After(out) {
			out = t
		}
	}
	return out
}

func norm(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// State is what a volcano's levels say.
type State int

// The states a volcano's levels can give.
const (
	Unknown  State = iota // missing, null or unrecognized: neither stored nor ended
	Quiet                 // at normal: not a warning
	Elevated              // above normal: a warning
)

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
// aviation color code. A level it does not recognize is Unknown, never
// Quiet, so a feed change cannot end every warning at once.
func Classify(alertLevel, colorCode string) (severity.Level, State) {
	ground, color := norm(alertLevel), norm(colorCode)
	if l, ok := groundLevels[ground]; ok {
		return l, Elevated
	}
	switch ground {
	case "NORMAL":
		return 0, Quiet
	case "UNASSIGNED":
		if l, ok := aviationColors[color]; ok {
			return l, Elevated
		}
		if color == "GREEN" || color == "UNASSIGNED" {
			return 0, Quiet
		}
	}
	return 0, Unknown
}

// sourceSeverity is both of USGS's levels, as USGS names them: "Advisory
// · aviation Yellow".
func sourceSeverity(alertLevel, colorCode string) string {
	var parts []string
	if a := norm(alertLevel); a != "" && a != "UNASSIGNED" {
		parts = append(parts, title(a))
	}
	if c := norm(colorCode); c != "" && c != "UNASSIGNED" {
		parts = append(parts, "aviation "+title(c))
	}
	return strings.Join(parts, " · ")
}

func title(s string) string {
	return s[:1] + strings.ToLower(s[1:])
}
