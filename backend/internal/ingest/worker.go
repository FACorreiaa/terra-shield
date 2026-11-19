package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"shield/internal/fetch"
	"shield/internal/metrics"

	"github.com/riverqueue/river"
)

// PollArgs is the River job that polls one source.
type PollArgs struct {
	Source string `json:"source"`
}

// Kind is the River job kind.
func (PollArgs) Kind() string { return "poll_source" }

const (
	// maxSnooze caps a source's Retry-After so a bad header cannot park a
	// source for days; the staleness alert fires long before this.
	maxSnooze = time.Hour
	// pollTimeout bounds one poll, fetch and store together. FIRMS' global
	// CSVs are the slowest.
	pollTimeout = 3 * time.Minute
	// pollMaxAttempts keeps retries inside one poll interval for the fast
	// feeds; the next scheduled poll supersedes a failing one anyway.
	pollMaxAttempts = 3
)

// PollWorker runs PollArgs jobs.
type PollWorker struct {
	river.WorkerDefaults[PollArgs]
	sources map[string]Source
	states  SourceStates
	logger  *slog.Logger
	now     func() time.Time
}

// NewPollWorker builds the worker for the given sources.
func NewPollWorker(states SourceStates, logger *slog.Logger, sources []Source) *PollWorker {
	return &PollWorker{sources: byID(sources), states: states, logger: logger, now: time.Now}
}

// Timeout bounds each job.
func (*PollWorker) Timeout(*river.Job[PollArgs]) time.Duration { return pollTimeout }

// Work polls one source and records the outcome.
func (w *PollWorker) Work(ctx context.Context, job *river.Job[PollArgs]) error {
	id := job.Args.Source
	src, ok := w.sources[id]
	if !ok {
		// A job left over from a removed source: retrying cannot help.
		return river.JobCancel(fmt.Errorf("unknown source %q", id))
	}

	sent, err := w.states.Validators(ctx, id)
	if err != nil {
		return err
	}
	res, pollErr := src.Poll(ctx, sent)
	now := w.now().UTC()
	if pollErr != nil {
		return w.failed(ctx, job, id, now, pollErr)
	}

	// A 304 often carries no validators; keep the ones that earned it.
	if res.NotModified && res.Validators.IsZero() {
		res.Validators = sent
	}
	result := "ok"
	if res.NotModified {
		result = "not_modified"
	}
	metrics.SourcePolls.WithLabelValues(id, result).Inc()
	metrics.SourceEventsUpserted.WithLabelValues(id).Add(float64(res.Upserted))
	metrics.SourceLastSuccess.WithLabelValues(id).Set(float64(now.Unix()))
	if updated, ok := sourceUpdatedAt(&res); ok {
		metrics.SourceLag.WithLabelValues(id).Set(max(0, now.Sub(updated).Seconds()))
	}

	if err := w.states.RecordSuccess(ctx, id, now, res.Validators); err != nil {
		return fmt.Errorf("record success: %w", err)
	}
	w.logger.Info("polled", "source", id, "result", result, "upserted", res.Upserted)
	return nil
}

// sourceUpdatedAt is the adapter's own timestamp, else the feed's
// Last-Modified (kept across 304s, so a quiet feed's lag keeps growing).
func sourceUpdatedAt(res *Result) (time.Time, bool) {
	if !res.SourceUpdatedAt.IsZero() {
		return res.SourceUpdatedAt, true
	}
	if res.Validators.LastModified == "" {
		return time.Time{}, false
	}
	t, err := http.ParseTime(res.Validators.LastModified)
	return t, err == nil
}

func (w *PollWorker) failed(ctx context.Context, job *river.Job[PollArgs], id string, now time.Time, pollErr error) error {
	metrics.SourcePolls.WithLabelValues(id, "error").Inc()
	if err := w.states.RecordError(context.WithoutCancel(ctx), id, now, pollErr.Error()); err != nil {
		w.logger.Error("record source error", "source", id, "error", err)
	}
	w.logger.Warn("poll failed", "source", id, "attempt", job.Attempt, "error", pollErr)
	// The source asked us to wait: honor it instead of River's own backoff
	// (ADR 16: being blocked is the worst latency of all).
	var se *fetch.StatusError
	if errors.As(pollErr, &se) && se.RetryAfter > 0 {
		return river.JobSnooze(min(se.RetryAfter, maxSnooze))
	}
	return pollErr
}

// HousekeepArgs is the River job that runs one source's Housekeep.
type HousekeepArgs struct {
	Source string `json:"source"`
}

// Kind is the River job kind.
func (HousekeepArgs) Kind() string { return "housekeep_source" }

// HousekeepWorker runs HousekeepArgs jobs.
type HousekeepWorker struct {
	river.WorkerDefaults[HousekeepArgs]
	sources map[string]Source
	logger  *slog.Logger
}

// Work runs the source's cleanup.
func (w *HousekeepWorker) Work(ctx context.Context, job *river.Job[HousekeepArgs]) error {
	hk, ok := w.sources[job.Args.Source].(Housekeeper)
	if !ok {
		return river.JobCancel(fmt.Errorf("source %q has no housekeeping", job.Args.Source))
	}
	if err := hk.Housekeep(ctx); err != nil {
		w.logger.Warn("housekeeping failed", "source", job.Args.Source, "error", err)
		return err
	}
	return nil
}

// periodicJobs schedules one poll per enabled source, plus housekeeping for
// those that need it. ByPeriod uniqueness stops a slow run and the next
// tick from overlapping.
func periodicJobs(sources []Source) []*river.PeriodicJob {
	var jobs []*river.PeriodicJob
	for _, s := range sources {
		info := s.Info()
		if !info.Enabled {
			continue
		}
		jobs = append(jobs, periodic("poll:"+info.ID, info.PollInterval, PollArgs{Source: info.ID}, pollMaxAttempts))
		if hk, ok := s.(Housekeeper); ok {
			jobs = append(jobs, periodic("housekeep:"+info.ID, hk.HousekeepInterval(), HousekeepArgs{Source: info.ID}, 1))
		}
	}
	return jobs
}

func periodic(id string, every time.Duration, args river.JobArgs, attempts int) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(every),
		func() (river.JobArgs, *river.InsertOpts) {
			return args, &river.InsertOpts{
				MaxAttempts: attempts,
				UniqueOpts:  river.UniqueOpts{ByArgs: true, ByPeriod: every},
			}
		},
		&river.PeriodicJobOpts{ID: id, RunOnStart: true},
	)
}

func byID(sources []Source) map[string]Source {
	m := make(map[string]Source, len(sources))
	for _, s := range sources {
		m[s.Info().ID] = s
	}
	return m
}
