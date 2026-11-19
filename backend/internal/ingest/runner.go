package ingest

import (
	"context"
	"log/slog"
	"time"

	"shield/internal/domain"
	"shield/internal/metrics"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// stopBudget is how long running jobs get to finish on SIGTERM, inside the
// chart's grace period.
const stopBudget = 20 * time.Second

// Runner owns the River client that schedules every source.
type Runner struct {
	pool    *pgxpool.Pool // River's queue lives in the same database
	states  SourceStates
	sources []Source
	logger  *slog.Logger
}

// NewRunner builds a Runner.
func NewRunner(pool *pgxpool.Pool, states SourceStates, sources []Source, logger *slog.Logger) *Runner {
	return &Runner{pool: pool, states: states, sources: sources, logger: logger}
}

// seedHealth exports a source's poll interval and last success before its
// first poll, so the staleness alert sees every source from the start. A
// source that never succeeded counts from this start: one failing since
// boot goes stale after three intervals instead of exporting nothing.
func (r *Runner) seedHealth(ctx context.Context, info domain.SourceInfo, started time.Time) error {
	metrics.SourcePollInterval.WithLabelValues(info.ID).Set(info.PollInterval.Seconds())
	last, err := r.states.LastSuccess(ctx, info.ID)
	if err != nil {
		return err
	}
	at := started
	if last != nil {
		at = *last
	}
	metrics.SourceLastSuccess.WithLabelValues(info.ID).Set(float64(at.Unix()))
	return nil
}

// Run registers source metadata, starts River and blocks until ctx ends,
// then stops gracefully.
func (r *Runner) Run(ctx context.Context) error {
	started := time.Now()
	for _, s := range r.sources {
		info := s.Info()
		if err := r.states.Register(ctx, info); err != nil {
			return err
		}
		if err := r.seedHealth(ctx, info, started); err != nil {
			return err
		}
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, NewPollWorker(r.states, r.logger, r.sources))
	river.AddWorker(workers, &HousekeepWorker{sources: byID(r.sources), logger: r.logger})
	client, err := river.NewClient(riverpgxv5.New(r.pool), &river.Config{
		Logger:       r.logger,
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		Workers:      workers,
		PeriodicJobs: periodicJobs(r.sources),
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	r.logger.Info("worker started", "sources", len(r.sources))
	<-ctx.Done()

	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopBudget)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		r.logger.Warn("river stop timed out; canceling running jobs", "error", err)
		hardCtx, hardCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer hardCancel()
		return client.StopAndCancel(hardCtx)
	}
	return nil
}
