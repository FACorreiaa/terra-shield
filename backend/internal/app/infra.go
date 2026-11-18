package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"shield/internal/config"
	"shield/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// container is the infrastructure every database-backed command shares.
type container struct {
	cfg    config.Base
	logger *slog.Logger
	pool   *pgxpool.Pool
	repos  *store.Repositories
}

// open connects to the database and builds the repositories. Close the
// container when done.
func open(ctx context.Context, cfg config.Base, logger *slog.Logger) (*container, error) {
	pool, err := openDB(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &container{cfg: cfg, logger: logger, pool: pool, repos: store.New(pool)}, nil
}

func (c *container) Close() { c.pool.Close() }

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
