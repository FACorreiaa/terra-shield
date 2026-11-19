package app

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"shield/internal/config"
	"shield/internal/geo/countries"
	"shield/internal/geo/dataset"
	"shield/internal/geo/warningareas"
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
	if err != nil {
		return err
	}
	if n == 0 {
		logger.Info("countries table empty; seeding")
		if err := seedCountries(ctx, c.repos.Countries, logger, ""); err != nil {
			return err
		}
	}
	// MeteoAlarm areas reload whenever their pin changes. They are not
	// worth blocking a rollout over: without them only code-only warnings
	// are skipped, and the next migrate retries.
	if current, err := warningAreasCurrent(ctx, c.repos.WarningAreas); err != nil || !current {
		if err := seedWarningAreas(ctx, c.repos.WarningAreas, logger, "", ""); err != nil {
			logger.Warn("meteoalarm areas not seeded; code-only warnings are skipped until they are", "error", err)
		}
	}
	return nil
}

// warningAreasCurrent reports whether both MeteoAlarm files are loaded at
// their pinned versions.
func warningAreasCurrent(ctx context.Context, repo *store.WarningAreaRepository) (bool, error) {
	for _, pin := range []dataset.Pin{warningareas.Geocodes, warningareas.Aliases} {
		v, err := repo.DatasetVersion(ctx, pin.Name)
		if err != nil || v != warningareas.Version(pin) {
			return false, err
		}
	}
	return true, nil
}

func seedWarningAreasCmd(ctx context.Context, cfg config.Base, logger *slog.Logger, file, aliases string) error {
	c, err := open(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer c.Close()
	return seedWarningAreas(ctx, c.repos.WarningAreas, logger, file, aliases)
}

// seedWarningAreas loads the pinned MeteoAlarm geocodes and aliases, from
// files when given, else downloaded; both verify the SHA-256.
func seedWarningAreas(ctx context.Context, repo *store.WarningAreaRepository, logger *slog.Logger, file, aliasesFile string) error {
	client := &http.Client{Timeout: 3 * time.Minute}
	geo, err := warningareas.Geocodes.Read(ctx, client, file)
	if err != nil {
		return err
	}
	csv, err := warningareas.Aliases.Read(ctx, client, aliasesFile)
	if err != nil {
		return err
	}
	areas, err := warningareas.ParseGeocodes(geo)
	if err != nil {
		return err
	}
	aliases, err := warningareas.ParseAliases(csv, areas)
	if err != nil {
		return err
	}
	start := time.Now()
	versions := map[string]string{
		warningareas.Geocodes.Name: warningareas.Version(warningareas.Geocodes),
		warningareas.Aliases.Name:  warningareas.Version(warningareas.Aliases),
	}
	if err := repo.Load(ctx, areas, aliases, warningareas.SimplifyDegrees, versions); err != nil {
		return err
	}
	logger.Info("meteoalarm areas seeded", "areas", len(areas), "aliases", len(aliases), "duration", time.Since(start))
	return nil
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
	body, err := countries.Dataset.Read(ctx, &http.Client{Timeout: 2 * time.Minute}, file)
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
