// Package app is Shield's composition root. It parses the command line,
// builds infrastructure (config, logger, database pool, repositories,
// HTTP clients) once, and injects it into the server, the features and
// the ingest worker. Nothing below this package constructs its own
// dependencies.
//
//	shield serve [--port 8080]       HTTP API
//	shield worker                    River ingestion worker (+ /metrics on METRICS_PORT)
//	shield migrate [--yes] [--no-seed] | status
//	                                 goose + River migrations; seeds countries when empty
//	                                 and MeteoAlarm areas when their pin changed
//	shield seed-countries [--file f] (re)load Natural Earth boundaries
//	shield seed-warning-areas [--file f --aliases f]
//	                                 (re)load MeteoAlarm warning areas
//	shield routes                    print every route, for the spec test
package app

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"shield/internal/config"
)

// options are the command-line flags; each command reads the ones it uses.
type options struct {
	port          int
	seed          bool
	countriesFile string // also the geocodes file for seed-warning-areas
	aliasesFile   string
	args          []string // positional arguments after the flags
}

// Run executes one command. version is the build's git SHA.
func Run(ctx context.Context, argv []string, version string) error {
	cmd, opts, err := parseArgs(argv)
	if err != nil {
		return err
	}
	cfg, err := config.Load[config.Base]()
	if err != nil {
		return err
	}
	if opts.port != 0 {
		cfg.Port = opts.port
	}
	if cfg.GitSHA == "" {
		cfg.GitSHA = version
	}
	logger := newLogger(cfg)

	switch cmd {
	case "serve":
		return serve(ctx, cfg, logger)
	case "worker":
		return worker(ctx, cfg, logger)
	case "migrate":
		return migrateCmd(ctx, cfg, logger, opts)
	case "seed-countries":
		return seedCountriesCmd(ctx, cfg, logger, opts.countriesFile)
	case "seed-warning-areas":
		return seedWarningAreasCmd(ctx, cfg, logger, opts.countriesFile, opts.aliasesFile)
	case "routes":
		return printRoutes(cfg)
	}
	return fmt.Errorf("unknown command %q", cmd)
}

func parseArgs(argv []string) (string, options, error) {
	cmd := "serve"
	if len(argv) > 0 && !strings.HasPrefix(argv[0], "-") {
		cmd, argv = argv[0], argv[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	var o options
	fs.IntVar(&o.port, "port", 0, "listen port (overrides PORT)")
	_ = fs.Bool("yes", false, "accepted for the chart's migrate job; migrations never prompt")
	noSeed := fs.Bool("no-seed", false, "migrate: do not seed countries when the table is empty")
	fs.StringVar(&o.countriesFile, "file", "", "seed-countries, seed-warning-areas: read this GeoJSON instead of downloading the pinned dataset")
	fs.StringVar(&o.aliasesFile, "aliases", "", "seed-warning-areas: read this alias CSV instead of downloading the pinned one")
	if err := fs.Parse(argv); err != nil {
		return "", options{}, err
	}
	o.seed = !*noSeed
	o.args = fs.Args()
	return cmd, o, nil
}
