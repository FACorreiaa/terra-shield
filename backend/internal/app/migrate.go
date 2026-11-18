package app

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"shield/internal/config"
	"shield/internal/geo/countries"
	"shield/internal/migrate"
	"shield/internal/store"
)

func migrateCmd(ctx context.Context, cfg config.Base, logger *slog.Logger, o options) error {
	c, err := open(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer c.Close()
	if len(o.args) > 0 && o.args[0] == "status" {
		return migrate.Status(ctx, c.pool, logger)
	}
	if err := migrate.Up(ctx, c.pool, logger); err != nil {
		return err
	}
	if !o.seed {
		return nil
	}
	// First rollout on a fresh database: load borders so country
	// assignment works from the first poll. Later rollouts skip this.
	n, err := c.repos.Countries.Count(ctx)
	if err != nil || n > 0 {
		return err
	}
	logger.Info("countries table empty; seeding")
	return seedCountries(ctx, c.repos.Countries, logger, "")
}

func seedCountriesCmd(ctx context.Context, cfg config.Base, logger *slog.Logger, file string) error {
	c, err := open(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer c.Close()
	return seedCountries(ctx, c.repos.Countries, logger, file)
}

// seedCountries loads the pinned Natural Earth dataset, from file when
// given (offline), else downloaded. Both paths verify the SHA-256.
func seedCountries(ctx context.Context, repo *store.CountryRepository, logger *slog.Logger, file string) error {
	var body []byte
	var err error
	if file != "" {
		body, err = os.ReadFile(file)
		if err == nil {
			err = countries.Verify(body, countries.DatasetSHA256)
		}
	} else {
		body, err = countries.Fetch(ctx, &http.Client{Timeout: 2 * time.Minute}, countries.DatasetURL, countries.DatasetSHA256)
	}
	if err != nil {
		return fmt.Errorf("countries dataset: %w", err)
	}
	parsed, skipped, err := countries.Parse(bytes.NewReader(body))
	if err != nil {
		return err
	}
	start := time.Now()
	n, err := repo.Load(ctx, parsed)
	if err != nil {
		return err
	}
	logger.Info("countries seeded", "countries", n, "features", len(parsed), "skipped", skipped, "duration", time.Since(start))
	return nil
}
