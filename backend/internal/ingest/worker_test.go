package ingest

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"shield/internal/store/sourcesdb"
	"shield/internal/testutil"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type fakeSource struct {
	info Info
	got  Validators
	res  Result
	err  error
}

func (f *fakeSource) Info() Info { return f.info }

func (f *fakeSource) Poll(_ context.Context, v Validators) (Result, error) {
	f.got = v
	return f.res, f.err
}

func job(source string) *river.Job[PollArgs] {
	return &river.Job[PollArgs]{JobRow: &rivertype.JobRow{Attempt: 1}, Args: PollArgs{Source: source}}
}

func setup(t *testing.T, src *fakeSource) (*PollWorker, *sourcesdb.Queries) {
	t.Helper()
	pool := testutil.DB(t)
	q := sourcesdb.New(pool)
	if err := q.UpsertSource(context.Background(), sourcesdb.UpsertSourceParams{
		ID: src.info.ID, Name: "Fake", Url: "https://example.org", License: "PD",
		Attribution: "Fake", PollIntervalSeconds: 60, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	w := NewPollWorker(pool, slog.New(slog.DiscardHandler), []Source{src})
	fixed := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	w.now = func() time.Time { return fixed }
	return w, q
}

func TestPollRecordsSuccessAndValidators(t *testing.T) {
	src := &fakeSource{info: Info{ID: "fake"}, res: Result{Validators: Validators{ETag: `"v1"`}, Upserted: 3}}
	w, q := setup(t, src)
	ctx := context.Background()

	if err := w.Work(ctx, job("fake")); err != nil {
		t.Fatal(err)
	}
	v, err := q.GetSourceValidators(ctx, "fake")
	if err != nil {
		t.Fatal(err)
	}
	if v.Etag == nil || *v.Etag != `"v1"` {
		t.Fatalf("etag %v", v.Etag)
	}

	// The next poll sends the stored validator; a bare 304 keeps it.
	src.res = Result{NotModified: true}
	if err := w.Work(ctx, job("fake")); err != nil {
		t.Fatal(err)
	}
	if src.got.ETag != `"v1"` {
		t.Fatalf("sent etag %q", src.got.ETag)
	}
	v, _ = q.GetSourceValidators(ctx, "fake")
	if v.Etag == nil || *v.Etag != `"v1"` {
		t.Fatalf("etag after 304 %v", v.Etag)
	}

	rows, err := q.ListSources(ctx)
	if err != nil || len(rows) != 1 || rows[0].LastSuccessAt == nil {
		t.Fatalf("rows %+v err %v", rows, err)
	}
}

func TestPollRecordsError(t *testing.T) {
	src := &fakeSource{info: Info{ID: "fake"}, err: errors.New("feed returned 502")}
	w, q := setup(t, src)
	ctx := context.Background()

	if err := w.Work(ctx, job("fake")); err == nil {
		t.Fatal("want the poll error returned so River retries")
	}
	rows, err := q.ListSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].LastError == nil || *rows[0].LastError != "feed returned 502" || rows[0].LastErrorAt == nil {
		t.Fatalf("row %+v", rows[0])
	}
	if rows[0].LastSuccessAt != nil {
		t.Fatal("last_success_at set on failure")
	}
}

func TestUnknownSourceIsCancelled(t *testing.T) {
	src := &fakeSource{info: Info{ID: "fake"}}
	w, _ := setup(t, src)
	err := w.Work(context.Background(), job("gone"))
	var cancel *rivertype.JobCancelError
	if !errors.As(err, &cancel) {
		t.Fatalf("err %v, want JobCancelError", err)
	}
}

func TestPeriodicJobsSkipDisabled(t *testing.T) {
	jobs := periodicJobs([]Source{
		&fakeSource{info: Info{ID: "on", Enabled: true, PollInterval: time.Minute}},
		&fakeSource{info: Info{ID: "off", Enabled: false, PollInterval: time.Minute}},
	})
	if len(jobs) != 1 {
		t.Fatalf("got %d periodic jobs, want 1", len(jobs))
	}
}
