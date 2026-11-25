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
	// Bases is the military bases layer (L3).
	Bases *BaseRepository
	// Historic is the historic sites layer (L4).
	Historic *HistoricRepository
	// WarningAreas are MeteoAlarm's area shapes (S1.5).
	WarningAreas *WarningAreaRepository
	// NWSZones are NOAA's public forecast zones, for tsunami warnings.
	NWSZones *NWSZoneRepository
}

// New builds all repositories on pool.
func New(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		Events:    NewEventRepository(pool),
		Sources:   NewSourceRepository(pool),
		Countries: NewCountryRepository(pool),
		Fires:     NewFireRepository(pool),
		Bases:     NewBaseRepository(pool),
		Historic:  NewHistoricRepository(pool),

		WarningAreas: NewWarningAreaRepository(pool),
		NWSZones:     NewNWSZoneRepository(pool),
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
