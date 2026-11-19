package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"shield/internal/aircraft"
	"shield/internal/config"
	"shield/internal/features"
	"shield/internal/fetch"
	"shield/internal/realtime"
	"shield/internal/server"
	"shield/internal/sources/adsblol"
)

func serve(ctx context.Context, cfg config.Base, logger *slog.Logger) error {
	c, err := open(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer c.Close()

	hub, err := realtime.Start(ctx, c.repos.Events, logger, realtime.Options{})
	if err != nil {
		return err
	}
	// Live flights: the API calls the feed itself, on demand, through a
	// cache (the worker never polls it). Its own client, so the 1 s gap
	// the hazard feeds share does not slow a viewport's cells.
	flightsHTTP := fetch.New(fetch.Options{UserAgent: c.userAgent(), Timeout: 8 * time.Second, MaxBody: 8 << 20, HostGap: 200 * time.Millisecond})
	flightOpts := aircraft.Options{Logger: logger, Health: c.repos.Sources, SourceID: adsblol.ID}
	flights := aircraft.New(adsblol.New(flightsHTTP, cfg.AircraftFeedURL), flightOpts)
	if err := c.repos.Sources.Register(ctx, adsblol.Info(flights.TTL())); err != nil {
		return fmt.Errorf("register flights source: %w", err)
	}

	health := server.NewHealth()
	server.AddDatabaseCheck(health, c.pool, logger)
	srv, err := server.New(
		&server.Deps{Config: cfg, Logger: logger, Health: health},
		features.All(features.Deps{
			Countries: c.repos.Countries, Events: c.repos.Events, Fires: c.repos.Fires, Sources: c.repos.Sources,
			Stream: hub, Aircraft: flights, Logger: logger,
		}),
	)
	if err != nil {
		return err
	}
	logger.Info("listening", "port", cfg.Port, "env", cfg.Environment)
	return listen(ctx, logger, newHTTPServer(cfg.Port, srv.Handler))
}

// Routes lists every registered "METHOD /path", sorted. It builds the
// server without a database, so it runs anywhere (the spec test, CI).
func Routes(cfg config.Base) ([]string, error) {
	d := &server.Deps{Config: cfg, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	srv, err := server.New(d, features.All(features.Deps{}))
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, r := range srv.Echo.Router().Routes() {
		lines = append(lines, r.Method+" "+r.Path)
	}
	sort.Strings(lines)
	return lines, nil
}

func printRoutes(cfg config.Base) error {
	lines, err := Routes(cfg)
	if err != nil {
		return err
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return err
		}
	}
	return nil
}
