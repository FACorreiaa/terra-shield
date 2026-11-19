package events

import (
	"encoding/json"
	"maps"
	"time"

	"shield/internal/domain"
	"shield/internal/features/sources"
)

// Coordinate is a WGS 84 point.
type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Country is one country an event touches.
type Country struct {
	ISO2       string          `json:"iso2"`
	Relation   domain.Relation `json:"relation"`
	DistanceKm *float64        `json:"distanceKm,omitempty"` // nearby only
}

// Event is the wire shape of one event (openapi.yaml#/components/schemas/Event).
type Event struct {
	ID                  string           `json:"id"`
	Source              string           `json:"source"`
	Type                domain.EventType `json:"type"`
	Kind                domain.Kind      `json:"kind"`
	Status              domain.Status    `json:"status"`
	Severity            string           `json:"severity"`
	SourceSeverity      string           `json:"sourceSeverity,omitempty"`
	SeverityProvisional bool             `json:"severityProvisional,omitempty"`
	Title               string           `json:"title"`
	Summary             string           `json:"summary,omitempty"`
	Centroid            Coordinate       `json:"centroid"`
	Countries           []Country        `json:"countries"`
	StartedAt           *time.Time       `json:"startedAt,omitempty"`
	ObservedAt          time.Time        `json:"observedAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
	EndedAt             *time.Time       `json:"endedAt,omitempty"`
	FetchedAt           time.Time        `json:"fetchedAt"`
	URL                 string           `json:"url,omitempty"`
	Attrs               map[string]any   `json:"attrs"`
	// DistanceKm is set only on a near query.
	DistanceKm *float64 `json:"distanceKm,omitempty"`
}

// Cluster is several events merged at low zoom.
type Cluster struct {
	Centroid    Coordinate     `json:"centroid"`
	Count       int            `json:"count"`
	MaxSeverity string         `json:"maxSeverity"`
	Types       map[string]int `json:"types"`
	// Severities counts members per severity, zero omitted, for a
	// cluster marker drawn in proportion.
	Severities map[string]int `json:"severities"`
	// Statuses counts members per status, and ActiveSeverities per
	// severity among active members only, so a view that includes
	// resolved events can still say how many are ongoing and how serious.
	Statuses         map[string]int `json:"statuses"`
	ActiveSeverities map[string]int `json:"activeSeverities"`
	BBox             [4]float64     `json:"bbox"`
}

// eventPage is the part of a list response that identifies its content
// (the ETag source).
type eventPage struct {
	Events    []Event   `json:"events"`
	Clusters  []Cluster `json:"clusters"`
	Truncated bool      `json:"truncated"`
}

type eventList struct {
	eventPage
	GeneratedAt time.Time `json:"generatedAt"`
}

type feature struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties Event           `json:"properties"`
}

type featureCollection struct {
	Type      string    `json:"type"`
	Features  []feature `json:"features"`
	Truncated bool      `json:"truncated"`
}

// provisionalKey is the attr adapters set; the API lifts it to the top level.
const provisionalKey = "severityProvisional"

// FromRecord is the wire shape of one stored event. Other features embed
// it (country summaries), so every endpoint describes an event alike.
func FromRecord(r *domain.EventRecord) Event {
	attrs := maps.Clone(r.Attrs)
	if attrs == nil {
		attrs = map[string]any{}
	}
	provisional, _ := attrs[provisionalKey].(bool)
	delete(attrs, provisionalKey)

	countries := make([]Country, 0, len(r.Countries))
	for _, c := range r.Countries {
		wc := Country{ISO2: c.ISO2, Relation: c.Relation}
		if c.Relation == domain.RelationNearby {
			d := c.DistanceKm
			wc.DistanceKm = &d
		}
		countries = append(countries, wc)
	}

	return Event{
		ID:                  r.ID,
		Source:              r.SourceID,
		Type:                r.Type,
		Kind:                r.Kind,
		Status:              r.Status,
		Severity:            r.Severity,
		SourceSeverity:      r.SourceSeverity,
		SeverityProvisional: provisional,
		Title:               r.Title,
		Summary:             r.Summary,
		Centroid:            Coordinate{Latitude: r.Latitude, Longitude: r.Longitude},
		Countries:           countries,
		StartedAt:           utc(r.StartedAt),
		ObservedAt:          r.ObservedAt.UTC(),
		UpdatedAt:           r.UpdatedAt.UTC(),
		EndedAt:             utc(r.EndedAt),
		FetchedAt:           r.FetchedAt.UTC(),
		URL:                 r.URL,
		Attrs:               attrs,
		DistanceKm:          r.DistanceKm,
	}
}

func toWireList(recs []domain.EventRecord) []Event {
	out := make([]Event, 0, len(recs))
	for i := range recs {
		out = append(out, FromRecord(&recs[i]))
	}
	return out
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// Revision is one recorded change (ADR 6).
type Revision struct {
	At      time.Time      `json:"at"`
	Changes map[string]any `json:"changes"`
}

// EventDetail is the wire shape of GET /v1/events/{id}.
type EventDetail struct {
	Event
	Geometry   json.RawMessage `json:"geometry"`
	SourceInfo sources.Source  `json:"sourceInfo"`
	Revisions  []Revision      `json:"revisions"`
}

func toWireDetail(d *domain.EventDetail, now time.Time) EventDetail {
	revs := make([]Revision, 0, len(d.Revisions))
	for _, r := range d.Revisions {
		revs = append(revs, Revision{At: r.At.UTC(), Changes: r.Changes})
	}
	return EventDetail{
		Event:      FromRecord(&d.EventRecord),
		Geometry:   d.Geometry,
		SourceInfo: sources.FromStatus(&d.Source, now),
		Revisions:  revs,
	}
}
