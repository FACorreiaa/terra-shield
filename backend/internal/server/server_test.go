package server_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"shield/internal/config"
	"shield/internal/httpx"
	"shield/internal/server"
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
