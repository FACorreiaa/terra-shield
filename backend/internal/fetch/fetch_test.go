package fetch

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConditionalGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v2"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v2"`)
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	c := New(Options{HostGap: time.Millisecond})

	r, err := c.Get(t.Context(), srv.URL, Validators{})
	if err != nil || string(r.Body) != "body" || r.Validators.ETag != `"v2"` {
		t.Fatalf("first: %+v %v", r, err)
	}
	r, err = c.Get(t.Context(), srv.URL, r.Validators)
	if err != nil || !r.NotModified || r.Body != nil || r.Validators.ETag != `"v2"` {
		t.Fatalf("second: %+v %v", r, err)
	}
}

// USGS ignores conditional headers; an unchanged Last-Modified on a 200 is
// still "not modified".
func TestSameValidatorOn200IsNotModified(t *testing.T) {
	const lm = "Mon, 16 Nov 2026 16:25:47 GMT"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Last-Modified", lm)
		_, _ = w.Write([]byte("same"))
	}))
	defer srv.Close()
	c := New(Options{HostGap: time.Millisecond})

	r, err := c.Get(t.Context(), srv.URL, Validators{LastModified: lm})
	if err != nil || !r.NotModified {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = c.Get(t.Context(), srv.URL, Validators{LastModified: "Mon, 16 Nov 2026 16:24:47 GMT"})
	if err != nil || r.NotModified || string(r.Body) != "same" {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestStatusErrorCarriesRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := New(Options{HostGap: time.Millisecond}).Get(t.Context(), srv.URL, Validators{})
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 429 || se.RetryAfter != 2*time.Minute {
		t.Fatalf("err %v", err)
	}
}

func TestBodyCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()

	_, err := New(Options{MaxBody: 10, HostGap: time.Millisecond}).Get(t.Context(), srv.URL, Validators{})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err %v", err)
	}
}

func TestHostGapSerializes(t *testing.T) {
	var inFlight, maxInFlight atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := inFlight.Add(1)
		if n > maxInFlight.Load() {
			maxInFlight.Store(n)
		}
		time.Sleep(10 * time.Millisecond)
		inFlight.Add(-1)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := New(Options{HostGap: 20 * time.Millisecond})

	start := time.Now()
	done := make(chan error, 3)
	for range 3 {
		go func() {
			_, err := c.Get(t.Context(), srv.URL, Validators{})
			done <- err
		}()
	}
	for range 3 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if maxInFlight.Load() != 1 {
		t.Fatalf("max in flight %d, want 1", maxInFlight.Load())
	}
	if el := time.Since(start); el < 40*time.Millisecond {
		t.Fatalf("3 requests took %s, want >= 2 gaps", el)
	}
}

func TestHead(t *testing.T) {
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
		}
		w.Header().Set("ETag", `"big"`)
		_, _ = w.Write([]byte("a big file"))
	}))
	defer srv.Close()
	c := New(Options{HostGap: time.Millisecond})

	v, err := c.Head(t.Context(), srv.URL)
	if err != nil || v.ETag != `"big"` || gets != 0 {
		t.Fatalf("head %+v err %v gets %d", v, err, gets)
	}
	if !SameVersion(Validators{ETag: `"big"`}, v) || SameVersion(Validators{ETag: `"old"`}, v) || SameVersion(Validators{}, v) {
		t.Fatal("SameVersion")
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"":                              0,
		"30":                            30 * time.Second,
		"Mon, 16 Nov 2026 12:01:00 GMT": time.Minute,
		"garbage":                       0,
	}
	for in, want := range cases {
		if got := parseRetryAfter(in, now); got != want {
			t.Errorf("%q = %s, want %s", in, got, want)
		}
	}
}
