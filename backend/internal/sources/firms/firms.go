// Package firms ingests NASA FIRMS active-fire detections (ADR 10): every
// VIIRS thermal anomaly of the last 24 h, worldwide, into the
// fire_detections rolling window. They are detections, never "wildfires":
// a hotspot can be a gas flare, a volcano or a farm burn.
//
// Feeds: the public global 24 h CSVs, one per satellite (no MAP_KEY
// needed). Each is a sliding window regenerated as passes are processed,
// so a poll repeats most rows; the fire repository inserts only new ones.
// The files ignore conditional GET but keep a stable ETag, so each poll
// sends a HEAD per file and downloads (6–7 MB each) only what changed.
//
// NRT detections arrive about 3 h after the satellite pass; the UI says so.
//
// License: NASA open data, no restrictions; attribution requested.
package firms

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/ingest"
)

const (
	// ID is the source key in the sources table.
	ID = "firms"
	// PollInterval: new passes land every few tens of minutes.
	PollInterval = 15 * time.Minute
	// Retention is the rolling window kept in fire_detections.
	Retention = 48 * time.Hour
)

// File is one satellite's 24 h CSV.
type File struct {
	Satellite string
	URL       string
}

// Files are the VIIRS 375 m products. MODIS (1 km) adds little on top.
var Files = []File{
	{"NOAA-20", "https://firms.modaps.eosdis.nasa.gov/data/active_fire/noaa-20-viirs-c2/csv/J1_VIIRS_C2_Global_24h.csv"},
	{"NOAA-21", "https://firms.modaps.eosdis.nasa.gov/data/active_fire/noaa-21-viirs-c2/csv/J2_VIIRS_C2_Global_24h.csv"},
	{"Suomi NPP", "https://firms.modaps.eosdis.nasa.gov/data/active_fire/suomi-npp-viirs-c2/csv/SUOMI_VIIRS_C2_Global_24h.csv"},
}

// Source is the FIRMS adapter.
type Source struct {
	files  []File
	client *fetch.Client
	store  ingest.FireStore
	now    func() time.Time

	// held keeps each file's validators across polls. The sources table
	// holds one set per source, and this source reads several files.
	mu   sync.Mutex
	held map[string]domain.Validators
}

// New builds the adapter on the production files.
func New(client *fetch.Client, store ingest.FireStore) *Source {
	return &Source{files: Files, client: client, store: store, now: time.Now, held: map[string]domain.Validators{}}
}

// Info implements ingest.Source.
func (*Source) Info() domain.SourceInfo {
	return domain.SourceInfo{
		ID:           ID,
		Name:         "NASA FIRMS (Fire Information for Resource Management System)",
		URL:          "https://firms.modaps.eosdis.nasa.gov/",
		License:      "NASA open data, no restrictions",
		Attribution:  "NASA FIRMS (Fire Information for Resource Management System), VIIRS 375 m NRT",
		Scope:        "Satellite fire detections worldwide (VIIRS 375 m), last 24 hours",
		TermsURL:     "https://www.earthdata.nasa.gov/engage/open-data-services-software-policies/data-use-guidance",
		Cadence:      "each satellite pass, about 3 hours behind",
		PollInterval: PollInterval,
		Enabled:      true,
	}
}

// Poll implements ingest.Source. Files are fetched in order; one failing
// file fails the poll, and its validators are not advanced, so the next
// poll retries it.
func (s *Source) Poll(ctx context.Context, _ domain.Validators) (ingest.Result, error) {
	var fires []domain.FireDetection
	fresh := map[string]domain.Validators{}
	for _, f := range s.files {
		v, body, changed, err := s.fetchIfChanged(ctx, f.URL)
		if err != nil {
			return ingest.Result{}, fmt.Errorf("%s: %w", f.Satellite, err)
		}
		if !changed {
			continue
		}
		parsed, err := Parse(bytes.NewReader(body), f.Satellite)
		if err != nil {
			return ingest.Result{}, fmt.Errorf("%s: %w", f.Satellite, err)
		}
		fires = append(fires, parsed...)
		fresh[f.URL] = v
	}
	if len(fresh) == 0 {
		return ingest.Result{NotModified: true, SourceUpdatedAt: s.newestModified()}, nil
	}
	n, err := s.store.Insert(ctx, fires)
	if err != nil {
		return ingest.Result{}, err
	}
	s.mu.Lock()
	for u, v := range fresh {
		s.held[u] = v
	}
	s.mu.Unlock()
	return ingest.Result{Upserted: n, SourceUpdatedAt: s.newestModified()}, nil
}

// newestModified is the latest Last-Modified among the files held: FIRMS
// is as fresh as its freshest satellite file. Zero when none parses.
func (s *Source) newestModified() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	var newest time.Time
	for _, v := range s.held {
		if t, err := http.ParseTime(v.LastModified); err == nil && t.After(newest) {
			newest = t
		}
	}
	return newest
}

func (s *Source) fetchIfChanged(ctx context.Context, url string) (domain.Validators, []byte, bool, error) {
	s.mu.Lock()
	held, ok := s.held[url]
	s.mu.Unlock()
	if ok {
		head, err := s.client.Head(ctx, url)
		if err != nil {
			return domain.Validators{}, nil, false, err
		}
		if fetch.SameVersion(held, head) {
			return held, nil, false, nil
		}
	}
	resp, err := s.client.Get(ctx, url, held)
	if err != nil {
		return domain.Validators{}, nil, false, err
	}
	if resp.NotModified {
		return resp.Validators, nil, false, nil
	}
	return resp.Validators, resp.Body, true, nil
}

// Housekeep implements ingest.Housekeeper: drop detections older than the
// retention window.
func (s *Source) Housekeep(ctx context.Context) error {
	_, err := s.store.PurgeBefore(ctx, s.now().Add(-Retention))
	return err
}

// HousekeepInterval implements ingest.Housekeeper.
func (*Source) HousekeepInterval() time.Duration { return time.Hour }

var requiredColumns = []string{"latitude", "longitude", "acq_date", "acq_time", "confidence", "frp"}

// Parse reads a FIRMS VIIRS CSV. Rows with unusable coordinates or times
// are skipped; a missing column fails the file, since that means the
// format changed.
func Parse(r io.Reader, satellite string) ([]domain.FireDetection, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true
	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil // an empty file: the satellite had no processed passes
	}
	if err != nil {
		return nil, fmt.Errorf("firms: header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.TrimSpace(h)] = i
	}
	for _, c := range requiredColumns {
		if _, ok := col[c]; !ok {
			return nil, fmt.Errorf("firms: column %q missing", c)
		}
	}

	var out []domain.FireDetection
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("firms: row: %w", err)
		}
		if d, ok := parseRow(rec, col, satellite); ok {
			out = append(out, d)
		}
	}
}

func parseRow(rec []string, col map[string]int, satellite string) (domain.FireDetection, bool) {
	lat, err1 := strconv.ParseFloat(rec[col["latitude"]], 64)
	lon, err2 := strconv.ParseFloat(rec[col["longitude"]], 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return domain.FireDetection{}, false
	}
	// acq_time is HHMM in UTC, zero-padded ("0011").
	at, err := time.Parse("2006-01-02 1504", rec[col["acq_date"]]+" "+fmt.Sprintf("%04s", rec[col["acq_time"]]))
	if err != nil {
		return domain.FireDetection{}, false
	}
	d := domain.FireDetection{Satellite: satellite, AcquiredAt: at.UTC(), Lat: lat, Lon: lon, Confidence: rec[col["confidence"]]}
	if frp, err := strconv.ParseFloat(rec[col["frp"]], 32); err == nil {
		v := float32(frp)
		d.FRP = &v
	}
	return d, true
}
