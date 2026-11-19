// Package features lists every API feature, in mount order, each built
// with the dependencies it declares.
package features

import (
	"log/slog"

	"shield/internal/features/aircraft"
	"shield/internal/features/countries"
	"shield/internal/features/events"
	"shield/internal/features/fires"
	"shield/internal/features/ops"
	"shield/internal/features/sources"
	"shield/internal/features/stream"
	"shield/internal/server"
)

// Deps is every repository a feature may need, as the narrow interface
// that feature declares. Nil fields are allowed: the routes command builds
// features without a database, and their handlers answer 503.
type Deps struct {
	Countries countries.Repository
	Events    events.Repository
	Fires     fires.Repository
	Sources   sources.Lister
	Stream    stream.Broker
	Aircraft  aircraft.Finder
	Logger    *slog.Logger
}

// All returns every feature.
func All(d Deps) []server.Feature {
	return []server.Feature{
		ops.New(),
		events.New(d.Events),
		countries.New(d.Countries),
		fires.New(d.Fires),
		sources.New(d.Sources),
		stream.New(d.Stream, d.Logger),
		aircraft.New(d.Aircraft),
	}
}
