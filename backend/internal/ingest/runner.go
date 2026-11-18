package ingest

import (
	"context"
	"log/slog"
	"time"

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

// Run registers source metadata, starts River and blocks until ctx ends,
// then stops gracefully.
func (r *Runner) Run(ctx context.Context) error {
	for _, s := range r.sources {
		if err := r.states.Register(ctx, s.Info()); err != nil {
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
