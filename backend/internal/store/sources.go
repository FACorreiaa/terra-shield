package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"shield/internal/domain"
	"shield/internal/store/sourcesdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SourceRepository keeps each feed's metadata and polling state.
type SourceRepository struct {
	pool *pgxpool.Pool
	q    *sourcesdb.Queries
}

// NewSourceRepository builds a SourceRepository.
func NewSourceRepository(pool *pgxpool.Pool) *SourceRepository {
	return &SourceRepository{pool: pool, q: sourcesdb.New(pool)}
}

// Register inserts or refreshes a source's metadata, keeping its state.
func (r *SourceRepository) Register(ctx context.Context, info domain.SourceInfo) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sourcesdb.New(tx)
	err = q.UpsertSource(ctx, sourcesdb.UpsertSourceParams{
		ID:                  info.ID,
		Name:                info.Name,
		Url:                 info.URL,
		License:             info.License,
		Attribution:         info.Attribution,
		PollIntervalSeconds: int32(info.PollInterval / time.Second), //nolint:gosec // poll intervals are minutes, far below int32
		Enabled:             info.Enabled,
		OfficialWarnings:    info.OfficialWarnings,
		Hazards:             !info.NonHazard,
	})
	if err != nil {
		return fmt.Errorf("register source %s: %w", info.ID, err)
	}
	// Coverage is replaced wholesale: the adapter's list is the truth.
	if err := q.ClearSourceCoverage(ctx, info.ID); err != nil {
		return fmt.Errorf("register source %s coverage: %w", info.ID, err)
	}
	if len(info.Coverage) > 0 {
		if _, err := q.AddSourceCoverage(ctx, sourcesdb.AddSourceCoverageParams{SourceID: info.ID, Iso2Codes: info.Coverage}); err != nil {
			return fmt.Errorf("register source %s coverage: %w", info.ID, err)
		}
	}
	return tx.Commit(ctx)
}

// Validators returns the validators from the source's last success; zero
// for an unknown or never-polled source.
func (r *SourceRepository) Validators(ctx context.Context, id string) (domain.Validators, error) {
	row, err := r.q.GetSourceValidators(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Validators{}, nil
	}
	if err != nil {
		return domain.Validators{}, fmt.Errorf("load validators %s: %w", id, err)
	}
	return domain.Validators{ETag: deref(row.Etag), LastModified: deref(row.LastModified)}, nil
}

// LastSuccess returns when the source last polled successfully; nil for
// an unknown or never-successful source.
func (r *SourceRepository) LastSuccess(ctx context.Context, id string) (*time.Time, error) {
	at, err := r.q.GetSourceLastSuccess(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load last success %s: %w", id, err)
	}
	return at, nil
}

// RecordSuccess stores a successful poll and the validators to send next.
func (r *SourceRepository) RecordSuccess(ctx context.Context, id string, at time.Time, v domain.Validators) error {
	return r.q.RecordSourceSuccess(ctx, sourcesdb.RecordSourceSuccessParams{
		ID:            id,
		LastSuccessAt: &at,
		Etag:          nonEmpty(v.ETag),
		LastModified:  nonEmpty(v.LastModified),
	})
}

// RecordError stores a failed poll.
func (r *SourceRepository) RecordError(ctx context.Context, id string, at time.Time, msg string) error {
	return r.q.RecordSourceError(ctx, sourcesdb.RecordSourceErrorParams{ID: id, LastError: &msg, LastErrorAt: &at})
}

// List returns every source with its health, ordered by id.
func (r *SourceRepository) List(ctx context.Context) ([]domain.SourceStatus, error) {
	rows, err := r.q.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SourceStatus, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.SourceStatus{
			SourceInfo: domain.SourceInfo{
				ID:           row.ID,
				Name:         row.Name,
				URL:          row.Url,
				License:      row.License,
				Attribution:  row.Attribution,
				PollInterval: time.Duration(row.PollIntervalSeconds) * time.Second,
				Enabled:      row.Enabled,
			},
			LastSuccessAt: row.LastSuccessAt,
			LastErrorAt:   row.LastErrorAt,
			LastError:     deref(row.LastError),
		})
	}
	return out, nil
}
