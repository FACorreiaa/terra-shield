package server_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shield/internal/config"
	"shield/internal/httpx"
	"shield/internal/server"

	"github.com/labstack/echo/v5"
)

func newServer(t *testing.T) http.Handler {
	t.Helper()
	d := &server.Deps{
		Config: config.Base{Environment: config.Production, RateLimitRPS: 20},
		Logger: slog.New(slog.DiscardHandler),
		Health: server.NewHealth(),
	}
	server.AddDatabaseCheck(d.Health, nil, d.Logger) // no database: readiness must fail
	srv, err := server.New(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv.Handler
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return rec
}

func TestLive(t *testing.T) {
	rec := get(t, newServer(t), "/health/live")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestReadyFailsWithoutDatabase(t *testing.T) {
	rec := get(t, newServer(t), "/health/ready")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "not_ready" {
		t.Fatalf("status %q", body.Status)
	}
}

func TestNotFoundUsesEnvelope(t *testing.T) {
	rec := get(t, newServer(t), "/v1/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	var env httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	if !env.Error || env.Code != "not_found" || env.RequestID == "" {
		t.Fatalf("envelope %+v", env)
	}
}

func TestMetrics(t *testing.T) {
	rec := get(t, newServer(t), "/metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWildcardOriginRefusedInProduction(t *testing.T) {
	d := &server.Deps{
		Config: config.Base{Environment: config.Production, AllowedOrigins: "*"},
		Logger: slog.New(slog.DiscardHandler),
		Health: server.NewHealth(),
	}
	if _, err := server.New(d, nil); err == nil {
		t.Fatal("want error for * in production")
	}
}

type slowFeature struct{}

func (slowFeature) Name() string { return "slow" }

func (slowFeature) Register(r *server.Routes) error {
	r.V1.GET("/ping", func(c *echo.Context) error { return c.String(http.StatusOK, "pong") })
	r.V1.GET("/slow", func(c *echo.Context) error {
		<-c.Request().Context().Done() // a query that honors its context
		return fmt.Errorf("query: %w", c.Request().Context().Err())
	})
	return nil
}

func hardened(t *testing.T, cfg config.Base) http.Handler {
	t.Helper()
	d := &server.Deps{Config: cfg, Logger: slog.New(slog.DiscardHandler), Health: server.NewHealth()}
	srv, err := server.New(d, []server.Feature{slowFeature{}})
	if err != nil {
		t.Fatal(err)
	}
	return srv.Handler
}

func TestRateLimitAnswers429WithRetryAfter(t *testing.T) {
	h := hardened(t, config.Base{RateLimitRPS: 1}) // burst 2
	codes := make([]int, 0, 3)
	var last *httptest.ResponseRecorder
	for range 3 {
		last = get(t, h, "/v1/ping")
		codes = append(codes, last.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("codes %v", codes)
	}
	var env httpx.Envelope
	if err := json.Unmarshal(last.Body.Bytes(), &env); err != nil || env.Code != "rate_limited" || last.Header().Get("Retry-After") != "1" {
		t.Fatalf("429 body %s retry=%q", last.Body, last.Header().Get("Retry-After"))
	}
	if get(t, h, "/health/live").Code != 200 {
		t.Fatal("probes must not be rate limited")
	}
}

func TestRequestTimeout(t *testing.T) {
	h := hardened(t, config.Base{RequestTimeout: 50 * time.Millisecond})
	start := time.Now()
	w := get(t, h, "/v1/slow")
	if w.Code != http.StatusServiceUnavailable || time.Since(start) > 2*time.Second {
		t.Fatalf("%d after %s: %s", w.Code, time.Since(start), w.Body)
	}
	var env httpx.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code != "unavailable" {
		t.Fatalf("body %s", w.Body)
	}
}

func TestCORS(t *testing.T) {
	h := hardened(t, config.Base{AllowedOrigins: "https://shield.example"})

	pre := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/v1/ping", nil)
	pre.Header.Set("Origin", "https://shield.example")
	pre.Header.Set("Access-Control-Request-Method", "GET")
	pre.Header.Set("Access-Control-Request-Headers", "if-none-match, last-event-id")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, pre)
	allowHeaders := strings.ToLower(w.Header().Get("Access-Control-Allow-Headers"))
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "https://shield.example" ||
		!strings.Contains(allowHeaders, "if-none-match") || !strings.Contains(allowHeaders, "last-event-id") ||
		w.Header().Get("Access-Control-Max-Age") != "600" || w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("preflight %d %v", w.Code, w.Header())
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/ping", nil)
	req.Header.Set("Origin", "https://shield.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if expose := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(expose, "ETag") || !strings.Contains(expose, "Retry-After") {
		t.Fatalf("expose %q", expose)
	}

	other := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/ping", nil)
	other.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, other)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unlisted origin allowed")
	}
}
