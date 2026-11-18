package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"shield/internal/domain"
	"shield/internal/store/countriesdb"

	"github.com/jackc/pgx/v5"
)

const (
	// summaryEvents bounds the events listed in a country summary.
	summaryEvents = 100
	// summaryChanges bounds the recent changes listed in a country summary.
	summaryChanges = 50
)

// List returns every country with its active-event stats, by name.
func (r *CountryRepository) List(ctx context.Context) ([]domain.CountryStats, error) {
	rows, err := countriesdb.New(r.pool).ListCountryStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("list countries: %w", err)
	}
	out := make([]domain.CountryStats, 0, len(rows))
	for i := range rows {
		out = append(out, countryStats(&rows[i]))
	}
	return out, nil
}

// Summary reads one country's stats, type × severity counts, coverage,
// active events and changes since `since`, all from one read-only
// snapshot so the parts agree. An unknown iso2 is domain.ErrNotFound.
func (r *CountryRepository) Summary(ctx context.Context, iso2 string, since time.Time) (domain.CountrySummary, error) {
	var s domain.CountrySummary
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return s, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := countriesdb.New(tx)

	stats, err := q.GetCountryStats(ctx, iso2)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, domain.ErrNotFound
	}
	if err != nil {
		return s, fmt.Errorf("country %s: %w", iso2, err)
	}
	s.Country = countryStats(&stats)

	counts, err := q.CountrySeverityCounts(ctx, iso2)
	if err != nil {
		return s, fmt.Errorf("country %s counts: %w", iso2, err)
	}
	s.Counts = make([]domain.SeverityCount, 0, len(counts))
	for _, c := range counts {
		s.Counts = append(s.Counts, domain.SeverityCount{Type: domain.EventType(c.Type), Severity: c.Severity, Count: int(c.Events)})
	}

	cov, err := q.CountryCoverage(ctx, iso2)
	if err != nil {
		return s, fmt.Errorf("country %s coverage: %w", iso2, err)
	}
	s.Coverage.Sources = make([]string, 0, len(cov))
	for _, c := range cov {
		s.Coverage.Sources = append(s.Coverage.Sources, c.ID)
		s.Coverage.OfficialWarnings = s.Coverage.OfficialWarnings || c.OfficialWarnings
	}

	if s.Events, _, err = listEvents(ctx, tx, domain.EventQuery{
		Country: iso2, Status: domain.StatusActive, Limit: summaryEvents,
	}); err != nil {
		return s, err
	}

	changes, err := q.CountryRecentChanges(ctx, countriesdb.CountryRecentChangesParams{Iso2: iso2, Since: since, MaxRows: summaryChanges})
	if err != nil {
		return s, fmt.Errorf("country %s changes: %w", iso2, err)
	}
	s.RecentChanges = make([]domain.EventChange, 0, len(changes))
	for _, c := range changes {
		ch := domain.EventChange{EventID: c.EventID.String(), Title: c.Title, Revision: domain.Revision{At: c.At}}
		if err := json.Unmarshal(c.Changes, &ch.Changes); err != nil {
			return s, fmt.Errorf("country %s change: %w", iso2, err)
		}
		s.RecentChanges = append(s.RecentChanges, ch)
	}
	return s, nil
}

func countryStats(r *countriesdb.CountryStat) domain.CountryStats {
	return domain.CountryStats{
		ISO2: r.Iso2, ISO3: r.Iso3, Name: r.Name,
		ActiveEvents: int(r.ActiveEvents), MaxSeverity: r.MaxSeverity,
		BBox: [4]float64{r.MinLon, r.MinLat, r.MaxLon, r.MaxLat},
	}
}
