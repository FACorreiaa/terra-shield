// Command shield is the Shield API server and ingestion worker.
//
//	shield serve [--port 8080]       HTTP API
//	shield worker                    River ingestion worker (+ /metrics on METRICS_PORT)
//	shield migrate [--yes] [--no-seed] | status
//	                                 goose + River migrations; seeds countries when empty
//	shield seed-countries [--file f] (re)load Natural Earth boundaries
//	shield routes                    print every route, for the spec test
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"shield/internal/config"
	"shield/internal/features"
	"shield/internal/geo/countries"
	"shield/internal/ingest"
	"shield/internal/metrics"
	"shield/internal/migrate"
	"shield/internal/server"

	"github.com/jackc/pgx/v5/pgxpool"
)

var gitSHA = "unknown"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	port := fs.Int("port", 0, "listen port (overrides PORT)")
	_ = fs.Bool("yes", false, "accepted for the chart's migrate job; migrations never prompt")
	noSeed := fs.Bool("no-seed", false, "migrate: do not seed countries when the table is empty")
	countriesFile := fs.String("file", "", "seed-countries: read this GeoJSON instead of downloading the pinned dataset")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load[config.Base]()
	if err != nil {
		return err
	}
	if *port != 0 {
		cfg.Port = *port
	}
	if cfg.GitSHA == "" {
		cfg.GitSHA = gitSHA
	}
	logger := newLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "serve":
		return serve(ctx, cfg, logger)
	case "worker":
		return worker(ctx, cfg, logger)
	case "migrate":
		return runMigrate(ctx, cfg, logger, fs.Args(), !*noSeed)
	case "seed-countries":
		return runSeedCountries(ctx, cfg, logger, *countriesFile)
	case "routes":
		return printRoutes(cfg)
	}
	return fmt.Errorf("unknown command %q", cmd)
}

func newLogger(cfg config.Base) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts)).With("version", cfg.Version())
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func openDB(ctx context.Context, cfg config.Base) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	// Retry for up to 30 s: on the cluster a fresh pod can get "connection
	// refused" until kube-router has applied the NetworkPolicy for its IP.
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		err := pool.Ping(pingCtx)
		if err == nil {
			return pool, nil
		}
		select {
		case <-pingCtx.Done():
			pool.Close()
			return nil, fmt.Errorf("database: %w", err)
		case <-time.After(time.Second):
		}
	}
}

// httpShutdownBudget leaves room inside the kubelet's 30 s grace period.
const httpShutdownBudget = 15 * time.Second

func serve(ctx context.Context, cfg config.Base, logger *slog.Logger) error {
	pool, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	d := &server.Deps{Config: cfg, Logger: logger, DB: pool, Health: server.NewHealth()}
	server.AddDatabaseCheck(d)
	srv, err := server.New(d, features.All())
	if err != nil {
		return err
	}
	logger.Info("listening", "port", cfg.Port, "env", cfg.Environment)
	return listen(ctx, logger, newHTTPServer(cfg.Port, srv.Handler))
}

func worker(ctx context.Context, cfg config.Base, logger *slog.Logger) error {
	pool, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The worker serves no API, only what the cluster probes and scrapes.
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", metrics.Handler())
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	opsCtx, stopOps := context.WithCancel(ctx)
	defer stopOps()
	opsErr := make(chan error, 1)
	go func() { opsErr <- listen(opsCtx, logger, newHTTPServer(cfg.MetricsPort, mux)) }()

	runErr := ingest.Run(ctx, pool, logger, registeredSources())
	stopOps()
	if err := <-opsErr; err != nil && runErr == nil {
		return err
	}
	return runErr
}

// listen serves until ctx ends, then shuts down within httpShutdownBudget.
func listen(ctx context.Context, logger *slog.Logger, s *http.Server) error {
	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe() }()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("shutting down", "addr", s.Addr)
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), httpShutdownBudget)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

// newHTTPServer limits header reads and idle connections. Read and write
// timeouts stay unset: the SSE stream (M2) is long-lived.
func newHTTPServer(port int, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func runMigrate(ctx context.Context, cfg config.Base, logger *slog.Logger, args []string, seed bool) error {
	pool, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	if len(args) > 0 && args[0] == "status" {
		return migrate.Status(ctx, pool, logger)
	}
	if err := migrate.Up(ctx, pool, logger); err != nil {
		return err
	}
	if !seed {
		return nil
	}
	// First rollout on a fresh database: load borders so country
	// assignment works from the first poll. Later rollouts skip this.
	n, err := countries.Count(ctx, pool)
	if err != nil || n > 0 {
		return err
	}
	logger.Info("countries table empty; seeding")
	return seedCountries(ctx, pool, logger, "")
}

func runSeedCountries(ctx context.Context, cfg config.Base, logger *slog.Logger, file string) error {
	pool, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	return seedCountries(ctx, pool, logger, file)
}

// seedCountries loads the pinned Natural Earth dataset, from file when
// given (offline), else downloaded. Both paths verify the SHA-256.
func seedCountries(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, file string) error {
	var body []byte
	var err error
	if file != "" {
		body, err = os.ReadFile(file) //nolint:gosec // operator-supplied path on a CLI command
		if err == nil {
			err = countries.Verify(body, countries.DatasetSHA256)
		}
	} else {
		client := &http.Client{Timeout: 2 * time.Minute}
		body, err = countries.Fetch(ctx, client, countries.DatasetURL, countries.DatasetSHA256)
	}
	if err != nil {
		return fmt.Errorf("countries dataset: %w", err)
	}
	features, skipped, err := countries.Parse(bytes.NewReader(body))
	if err != nil {
		return err
	}
	start := time.Now()
	n, err := countries.Load(ctx, pool, features)
	if err != nil {
		return err
	}
	logger.Info("countries seeded", "countries", n, "features", len(features), "skipped", skipped, "duration", time.Since(start))
	return nil
}

// Routes lists every registered "METHOD /path", sorted.
func Routes(cfg config.Base) ([]string, error) {
	d := &server.Deps{Config: cfg, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	srv, err := server.New(d, features.All())
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
		fmt.Println(l)
	}
	return nil
}
