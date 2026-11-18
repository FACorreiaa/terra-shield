package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSourceStatusStale(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	s := SourceStatus{SourceInfo: SourceInfo{Enabled: true, PollInterval: time.Minute}}
	if !s.Stale(now) {
		t.Error("never polled should be stale")
	}
	s.LastSuccessAt = at(3 * time.Minute)
	if s.Stale(now) {
		t.Error("exactly three intervals is not yet stale")
	}
	s.LastSuccessAt = at(3*time.Minute + time.Second)
	if !s.Stale(now) {
		t.Error("past three intervals should be stale")
	}
	s.Enabled = false
	if s.Stale(now) {
		t.Error("disabled is never stale")
	}
}

func TestPointGeometry(t *testing.T) {
	var g struct {
		Type        string     `json:"type"`
		Coordinates [2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(PointGeometry(-9.14, 38.72), &g); err != nil || g.Type != "Point" || g.Coordinates != [2]float64{-9.14, 38.72} {
		t.Fatalf("%+v err %v", g, err)
	}
}
