package store

import (
	"context"
	"fmt"

	"shield/internal/domain"
	"shield/internal/store/changesdb"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ChangeRepository reads what changed over a window.
type ChangeRepository struct {
	pool *pgxpool.Pool
}

// NewChangeRepository builds the repository.
func NewChangeRepository(pool *pgxpool.Pool) *ChangeRepository {
	return &ChangeRepository{pool: pool}
}

// List returns every event that changed since q.Since, one change each,
// unordered.
func (r *ChangeRepository) List(ctx context.Context, q domain.ChangeQuery) ([]domain.Change, error) {
	types := make([]string, len(q.Types))
	for i, t := range q.Types {
		types[i] = string(t)
	}
	severities := q.Severities
	if severities == nil {
		severities = []string{}
	}
	var iso2 *string
	if q.Country != "" {
		iso2 = &q.Country
	}
	rows, err := changesdb.New(r.pool).ListChanges(ctx, changesdb.ListChangesParams{
		Since: q.Since, Types: types, Severities: severities, Iso2: iso2,
	})
	if err != nil {
		return nil, fmt.Errorf("list changes: %w", err)
	}
	out := make([]domain.Change, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		if row.Change == "" {
			continue // touched in the window, but neither arrived, ended nor changed severity
		}
		c := domain.Change{
			EventID:   row.ID.String(),
			SourceID:  row.SourceID,
			Type:      domain.EventType(row.Type),
			Kind:      domain.Kind(row.Kind),
			Severity:  row.Severity,
			Title:     row.Title,
			Centroid:  [2]float64{row.Lon, row.Lat},
			Countries: row.Countries,
			StartedAt: row.StartedAt,
			Change:    domain.ChangeKind(row.Change),
			At:        row.At.UTC(),
		}
		if row.SourceSeverity != nil {
			c.SourceSeverity = *row.SourceSeverity
		}
		if c.Change == domain.ChangeEscalated || c.Change == domain.ChangeDowngraded {
			c.FromSeverity = row.FromSeverity
		}
		out = append(out, c)
	}
	return out, nil
}
