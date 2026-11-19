package app

import (
	"context"
	"log/slog"
	"net/http"

	"shield/internal/config"
	"shield/internal/ingest"
	"shield/internal/metrics"
	"shield/internal/retention"
)

func worker(ctx context.Context, cfg config.Base, logger *slog.Logger) error {
	c, err := open(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer c.Close()

	// The worker serves no API, only what the cluster probes and scrapes.
	opsCtx, stopOps := context.WithCancel(ctx)
	defer stopOps()
	opsErr := make(chan error, 1)
	go func() { opsErr <- listen(opsCtx, logger, newHTTPServer(cfg.MetricsPort, opsMux())) }()

	runErr := ingest.NewRunner(c.pool, c.repos.Sources, c.sources(), logger).
		WithTasks(retention.New(c.repos.Events, logger)).
		Run(ctx)
	stopOps()
	if err := <-opsErr; err != nil && runErr == nil {
		return err
	}
	return runErr
}

func opsMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", metrics.Handler())
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}
