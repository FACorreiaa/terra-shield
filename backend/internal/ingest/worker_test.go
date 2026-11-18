package ingest

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"shield/internal/domain"
	"shield/internal/fetch"
	"shield/internal/metrics"

	promtest "github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// fakeStates is an in-memory SourceStates.
type fakeStates struct {
	validators map[string]domain.Validators
	successAt  map[string]time.Time
	errors     map[string]string
}

func newFakeStates() *fakeStates {
	return &fakeStates{validators: map[string]domain.Validators{}, successAt: map[string]time.Time{}, errors: map[string]string{}}
}

func (f *fakeStates) Register(context.Context, domain.SourceInfo) error { return nil }

func (f *fakeStates) Validators(_ context.Context, id string) (domain.Validators, error) {
	return f.validators[id], nil
}

func (f *fakeStates) LastSuccess(_ context.Context, id string) (*time.Time, error) {
	if at, ok := f.successAt[id]; ok {
		return &at, nil
	}
	return nil, nil
}

func (f *fakeStates) RecordSuccess(_ context.Context, id string, at time.Time, v domain.Validators) error {
	f.successAt[id], f.validators[id] = at, v
	return nil
}

func (f *fakeStates) RecordError(_ context.Context, id string, _ time.Time, msg string) error {
	f.errors[id] = msg
	return nil
}

type fakeSource struct {
	info domain.SourceInfo
	got  domain.Validators
	res  Result
	err  error
}

func (f *fakeSource) Info() domain.SourceInfo { return f.info }

func (f *fakeSource) Poll(_ context.Context, v domain.Validators) (Result, error) {
	f.got = v
	return f.res, f.err
}

type housekeepingSource struct {
	fakeSource
	ran int
}

func (h *housekeepingSource) Housekeep(context.Context) error { h.ran++; return nil }
func (*housekeepingSource) HousekeepInterval() time.Duration  { return time.Hour }

func job(source string) *river.Job[PollArgs] {
	return &river.Job[PollArgs]{JobRow: &rivertype.JobRow{Attempt: 1}, Args: PollArgs{Source: source}}
}

func newWorker(src Source) (*PollWorker, *fakeStates) {
	states := newFakeStates()
	w := NewPollWorker(states, slog.New(slog.DiscardHandler), []Source{src})
	fixed := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	w.now = func() time.Time { return fixed }
	return w, states
}

func TestPollRecordsSuccessAndKeepsValidatorsOn304(t *testing.T) {
	src := &fakeSource{info: domain.SourceInfo{ID: "fake"}, res: Result{Validators: domain.Validators{ETag: `"v1"`}, Upserted: 3}}
	w, states := newWorker(src)
	ctx := context.Background()

	if err := w.Work(ctx, job("fake")); err != nil {
		t.Fatal(err)
	}
	if states.validators["fake"].ETag != `"v1"` || states.successAt["fake"].IsZero() {
		t.Fatalf("state %+v", states)
	}

	// The next poll sends the stored validator; a bare 304 keeps it.
	src.res = Result{NotModified: true}
	if err := w.Work(ctx, job("fake")); err != nil {
		t.Fatal(err)
	}
	if src.got.ETag != `"v1"` || states.validators["fake"].ETag != `"v1"` {
		t.Fatalf("sent %q, kept %q", src.got.ETag, states.validators["fake"].ETag)
	}
}

func TestPollRecordsError(t *testing.T) {
	src := &fakeSource{info: domain.SourceInfo{ID: "fake"}, err: errors.New("feed returned 502")}
	w, states := newWorker(src)

	if err := w.Work(context.Background(), job("fake")); err == nil {
		t.Fatal("want the poll error returned so River retries")
	}
	if states.errors["fake"] != "feed returned 502" || !states.successAt["fake"].IsZero() {
		t.Fatalf("state %+v", states)
	}
}

func TestRetryAfterSnoozes(t *testing.T) {
	src := &fakeSource{info: domain.SourceInfo{ID: "fake"}, err: &fetch.StatusError{Status: 429, RetryAfter: 2 * time.Minute}}
	w, _ := newWorker(src)
	var snooze *rivertype.JobSnoozeError
	if err := w.Work(context.Background(), job("fake")); !errors.As(err, &snooze) || snooze.Duration != 2*time.Minute {
		t.Fatalf("err %v, want a 2m snooze", err)
	}
}

func TestUnknownSourceIsCanceled(t *testing.T) {
	w, _ := newWorker(&fakeSource{info: domain.SourceInfo{ID: "fake"}})
	var cancel *rivertype.JobCancelError
	if err := w.Work(context.Background(), job("gone")); !errors.As(err, &cancel) {
		t.Fatalf("err %v, want JobCancelError", err)
	}
}

func TestPeriodicJobs(t *testing.T) {
	hk := &housekeepingSource{fakeSource: fakeSource{info: domain.SourceInfo{ID: "hk", Enabled: true, PollInterval: time.Minute}}}
	jobs := periodicJobs([]Source{
		&fakeSource{info: domain.SourceInfo{ID: "on", Enabled: true, PollInterval: time.Minute}},
		&fakeSource{info: domain.SourceInfo{ID: "off", Enabled: false, PollInterval: time.Minute}},
		hk,
	})
	// on: poll; hk: poll + housekeep; off: nothing.
	if len(jobs) != 3 {
		t.Fatalf("got %d periodic jobs, want 3", len(jobs))
	}

	w := &HousekeepWorker{sources: byID([]Source{hk}), logger: slog.New(slog.DiscardHandler)}
	if err := w.Work(context.Background(), &river.Job[HousekeepArgs]{Args: HousekeepArgs{Source: "hk"}}); err != nil || hk.ran != 1 {
		t.Fatalf("housekeep ran %d err %v", hk.ran, err)
	}
}

func TestPollReportsSourceLag(t *testing.T) {
	fixed := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC) // newWorker's clock
	lag := func(id string) float64 { return promtest.ToFloat64(metrics.SourceLag.WithLabelValues(id)) }

	// From the feed's Last-Modified, kept across a bare 304.
	src := &fakeSource{info: domain.SourceInfo{ID: "lag-lm"}, res: Result{Validators: domain.Validators{
		LastModified: fixed.Add(-90 * time.Second).Format(http.TimeFormat),
	}}}
	w, _ := newWorker(src)
	if err := w.Work(context.Background(), job("lag-lm")); err != nil {
		t.Fatal(err)
	}
	if got := lag("lag-lm"); got != 90 {
		t.Fatalf("Last-Modified lag %v, want 90", got)
	}
	src.res = Result{NotModified: true}
	w.now = func() time.Time { return fixed.Add(time.Minute) }
	if err := w.Work(context.Background(), job("lag-lm")); err != nil {
		t.Fatal(err)
	}
	if got := lag("lag-lm"); got != 150 {
		t.Fatalf("after a quiet 304 the lag should grow: %v, want 150", got)
	}

	// The adapter's own timestamp wins over Last-Modified.
	src2 := &fakeSource{info: domain.SourceInfo{ID: "lag-own"}, res: Result{
		SourceUpdatedAt: fixed.Add(-10 * time.Second),
		Validators:      domain.Validators{LastModified: fixed.Add(-time.Hour).Format(http.TimeFormat)},
	}}
	w2, _ := newWorker(src2)
	if err := w2.Work(context.Background(), job("lag-own")); err != nil {
		t.Fatal(err)
	}
	if got := lag("lag-own"); got != 10 {
		t.Fatalf("adapter lag %v, want 10", got)
	}

	// No timestamp at all (EONET): no series, rather than a made-up zero.
	src3 := &fakeSource{info: domain.SourceInfo{ID: "lag-none"}, res: Result{Validators: domain.Validators{LastModified: "not a date"}}}
	w3, _ := newWorker(src3)
	if err := w3.Work(context.Background(), job("lag-none")); err != nil {
		t.Fatal(err)
	}
	if n := promtest.CollectAndCount(metrics.SourceLag, "shield_source_lag_seconds"); n != 2 {
		t.Fatalf("%d lag series, want 2 (none for a source without a timestamp)", n)
	}
}
