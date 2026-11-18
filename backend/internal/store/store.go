// Package store is Shield's persistence layer: one repository per
// aggregate, each built on a pgxpool. Stable queries are sqlc-generated
// (the *db subpackages); bulk loads (COPY, temp staging tables) use pgx
// directly. Consumers never import this package's types in their own
// signatures: each declares the small interface it needs, and the
// composition root (internal/app) hands it a repository from here.
package store

import "github.com/jackc/pgx/v5/pgxpool"

// Repositories bundles every repository on one pool.
type Repositories struct {
	Events    *EventRepository
	Sources   *SourceRepository
	Countries *CountryRepository
	Fires     *FireRepository
}

// New builds all repositories on pool.
func New(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		Events:    NewEventRepository(pool),
		Sources:   NewSourceRepository(pool),
		Countries: NewCountryRepository(pool),
		Fires:     NewFireRepository(pool),
	}
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
