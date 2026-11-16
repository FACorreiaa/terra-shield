// Package migrate applies Shield's goose migrations, then River's own.
//
// Both run from `shield migrate --yes`, which the chart's Argo PreSync Job
// calls before a rollout, so neither the api nor the worker migrates at boot.
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	"shield/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

func provider(pool *pgxpool.Pool) (*goose.Provider, func() error, error) {
	sub, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return nil, nil, err
	}
	conn := stdlib.OpenDBFromPool(pool)
	p, err := goose.NewProvider(goose.DialectPostgres, conn, sub)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return p, conn.Close, nil
}

// Up applies pending goose migrations, then River's.
func Up(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	p, closeConn, err := provider(pool)
	if err != nil {
		return err
	}
	defer func() { _ = closeConn() }()
	results, err := p.Up(ctx)
	for _, r := range results {
		logger.Info("migration applied", "version", r.Source.Version, "path", r.Source.Path, "duration", r.Duration)
	}
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}

	migrator, err := rivermigrate.New[pgx.Tx](riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("river: %w", err)
	}
	for _, v := range res.Versions {
		logger.Info("river migration applied", "version", v.Version, "duration", v.Duration)
	}
	return nil
}

// Status logs each goose migration's state.
func Status(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	p, closeConn, err := provider(pool)
	if err != nil {
		return err
	}
	defer func() { _ = closeConn() }()
	statuses, err := p.Status(ctx)
	if err != nil {
		return err
	}
	for _, s := range statuses {
		logger.Info("migration", "version", s.Source.Version, "path", s.Source.Path, "state", s.State, "applied_at", s.AppliedAt)
	}
	return nil
}
