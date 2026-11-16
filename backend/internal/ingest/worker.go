package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"shield/internal/metrics"
	"shield/internal/store/sourcesdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// PollArgs is the River job that polls one source.
type PollArgs struct {
	Source string `json:"source"`
}

// Kind is the River job kind.
func (PollArgs) Kind() string { return "poll_source" }

// maxSnooze caps a source's Retry-After so a bad header cannot park a
// source for days; the staleness alert fires long before this.
const maxSnooze = time.Hour

// pollTimeout bounds one poll, fetch and store together. FIRMS' global CSV
// is the slowest feed.
const pollTimeout = 2 * time.Minute

// PollWorker runs PollArgs jobs.
type PollWorker struct {
	river.WorkerDefaults[PollArgs]
	sources map[string]Source
	queries *sourcesdb.Queries
	logger  *slog.Logger
	now     func() time.Time
}

// NewPollWorker builds the worker for the given sources.
func NewPollWorker(pool *pgxpool.Pool, logger *slog.Logger, sources []Source) *PollWorker {
	m := make(map[string]Source, len(sources))
	for _, s := range sources {
		m[s.Info().ID] = s
	}
	return &PollWorker{sources: m, queries: sourcesdb.New(pool), logger: logger, now: time.Now}
}

// Timeout bounds each job.
func (w *PollWorker) Timeout(*river.Job[PollArgs]) time.Duration { return pollTimeout }

// Work polls one source and records the outcome.
func (w *PollWorker) Work(ctx context.Context, job *river.Job[PollArgs]) error {
	id := job.Args.Source
	src, ok := w.sources[id]
	if !ok {
		// A job left over from a removed source: retrying cannot help.
		return river.JobCancel(fmt.Errorf("unknown source %q", id))
	}

	row, err := w.queries.GetSourceValidators(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load validators: %w", err)
	}
	sent := Validators{ETag: deref(row.Etag), LastModified: deref(row.LastModified)}
	res, pollErr := src.Poll(ctx, sent)
	now := w.now().UTC()
	// A 304 often carries no validators; keep the ones that earned it.
	if res.NotModified && res.Validators == (Validators{}) {
		res.Validators = sent
	}

	if pollErr != nil {
		metrics.SourcePolls.WithLabelValues(id, "error").Inc()
		msg := pollErr.Error()
		if err := w.queries.RecordSourceError(context.WithoutCancel(ctx), sourcesdb.RecordSourceErrorParams{
			ID: id, LastError: &msg, LastErrorAt: &now,
		}); err != nil {
			w.logger.Error("record source error", "source", id, "error", err)
		}
		w.logger.Warn("poll failed", "source", id, "attempt", job.Attempt, "error", pollErr)
		// The source asked us to wait: honour it instead of River's own
		// backoff (ADR 16: being blocked is the worst latency of all).
		var se *fetch.StatusError
		if errors.As(pollErr, &se) && se.RetryAfter > 0 {
			return river.JobSnooze(min(se.RetryAfter, maxSnooze))
		}
		return pollErr
	}

	result := "ok"
	if res.NotModified {
		result = "not_modified"
	}
	metrics.SourcePolls.WithLabelValues(id, result).Inc()
	metrics.SourceEventsUpserted.WithLabelValues(id).Add(float64(res.Upserted))
	metrics.SourceLastSuccess.WithLabelValues(id).Set(float64(now.Unix()))

	if err := w.queries.RecordSourceSuccess(ctx, sourcesdb.RecordSourceSuccessParams{
		ID:            id,
		LastSuccessAt: &now,
		Etag:          nonEmpty(res.Validators.ETag),
		LastModified:  nonEmpty(res.Validators.LastModified),
	}); err != nil {
		return fmt.Errorf("record success: %w", err)
	}
	w.logger.Info("polled", "source", id, "result", result, "upserted", res.Upserted)
	return nil
}

// pollMaxAttempts keeps retries inside one poll interval for the fast feeds;
// the next scheduled poll supersedes a failing one anyway.
const pollMaxAttempts = 3

// periodicJobs schedules one poll per enabled source. ByPeriod uniqueness
// stops a slow poll and the next tick from running side by side.
func periodicJobs(sources []Source) []*river.PeriodicJob {
	var jobs []*river.PeriodicJob
	for _, s := range sources {
		info := s.Info()
		if !info.Enabled {
			continue
		}
		jobs = append(jobs, river.NewPeriodicJob(
			river.PeriodicInterval(info.PollInterval),
			func() (river.JobArgs, *river.InsertOpts) {
				return PollArgs{Source: info.ID}, &river.InsertOpts{
					MaxAttempts: pollMaxAttempts,
					UniqueOpts:  river.UniqueOpts{ByArgs: true, ByPeriod: info.PollInterval},
				}
			},
			&river.PeriodicJobOpts{ID: "poll:" + info.ID, RunOnStart: true},
		))
	}
	return jobs
}

// stopBudget is how long running polls get to finish on SIGTERM, inside the
// chart's grace period.
const stopBudget = 20 * time.Second

// Run upserts source metadata, starts River and blocks until ctx ends.
func Run(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, sources []Source) error {
	q := sourcesdb.New(pool)
	for _, s := range sources {
		info := s.Info()
		if err := q.UpsertSource(ctx, sourcesdb.UpsertSourceParams{
			ID:                  info.ID,
			Name:                info.Name,
			Url:                 info.URL,
			License:             info.License,
			Attribution:         info.Attribution,
			PollIntervalSeconds: int32(info.PollInterval / time.Second), //nolint:gosec // poll intervals are minutes, far below int32
			Enabled:             info.Enabled,
		}); err != nil {
			return fmt.Errorf("upsert source %s: %w", info.ID, err)
		}
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, NewPollWorker(pool, logger, sources))
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:       logger,
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		Workers:      workers,
		PeriodicJobs: periodicJobs(sources),
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	logger.Info("worker started", "sources", len(sources))
	<-ctx.Done()

	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopBudget)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		logger.Warn("river stop timed out; canceling running jobs", "error", err)
		hardCtx, hardCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer hardCancel()
		return client.StopAndCancel(hardCtx)
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
