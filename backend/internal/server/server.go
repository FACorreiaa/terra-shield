// Package server assembles the Echo application: global middleware, health
// endpoints, /metrics, and every feature's routes.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"shield/internal/config"
	"shield/internal/httpx"
	"shield/internal/metrics"

	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
)

// Deps is what the server itself needs. Features get their own
// dependencies through their constructors (see internal/features).
type Deps struct {
	Config config.Base
	Logger *slog.Logger
	Health *Health
}

// Routes are the mount points: the root and /v1.
type Routes struct {
	Root *echo.Group
	V1   *echo.Group
}

// Feature is one API area. It is constructed with its dependencies
// (repositories behind small interfaces) and only mounts routes here.
type Feature interface {
	Name() string
	Register(r *Routes) error
}

// Server is the assembled HTTP handler.
type Server struct {
	Echo    *echo.Echo
	Handler http.Handler
}

// bodyLimit caps request bodies; the public API is read-only.
const bodyLimit = 64 * 1024

// New builds the server and mounts features.
func New(d *Deps, features []Feature) (*Server, error) {
	e := echo.NewWithConfig(echo.Config{
		Logger:           d.Logger,
		HTTPErrorHandler: httpx.ErrorHandler(d.Logger, d.Config.IsProduction()),
		// Traefik sets X-Forwarded-For; the default trust options accept
		// it only from loopback, link-local and private ranges.
		IPExtractor: echo.ExtractIPFromXFFHeader(),
	})

	origins, err := d.Config.Origins()
	if err != nil {
		return nil, err
	}

	e.Use(
		echomw.RequestID(),
		metrics.HTTPMiddleware(),
		echomw.Recover(),
		requestLog(d.Logger),
	)
	if len(origins) > 0 {
		e.Use(echomw.CORSWithConfig(corsConfig(origins)))
	}
	e.Use(
		echomw.GzipWithConfig(echomw.GzipConfig{
			MinLength: 1024,
			// A compressed event stream would sit in the gzip buffer.
			Skipper: isStream,
		}),
		echomw.BodyLimit(bodyLimit),
	)

	routes := &Routes{Root: e.Group(""), V1: e.Group("/v1")}
	registerOps(d, routes)

	// The rate limit and the timeout cover /v1 only: probes and scrapes
	// must never be throttled.
	if d.Config.RateLimitRPS > 0 {
		routes.V1.Use(rateLimit(d.Config.RateLimitRPS))
	}
	if d.Config.RequestTimeout > 0 {
		routes.V1.Use(echomw.ContextTimeoutWithConfig(echomw.ContextTimeoutConfig{
			Timeout: d.Config.RequestTimeout,
			// The stream is long-lived by design; its writes have their own
			// deadline.
			Skipper: isStream,
		}))
	}

	for _, f := range features {
		if err := f.Register(routes); err != nil {
			return nil, err
		}
	}
	return &Server{Echo: e, Handler: e}, nil
}

func isStream(c *echo.Context) bool { return c.Request().URL.Path == metrics.StreamRoute }

// corsConfig lets browser clients on the allowed origins read responses,
// revalidate with ETags and resume the stream. No credentials: the API is
// public and anonymous.
func corsConfig(origins []string) echomw.CORSConfig {
	return echomw.CORSConfig{
		AllowOrigins: origins,
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodOptions},
		AllowHeaders: []string{"Accept", "If-None-Match", "Last-Event-ID"},
		// Not CORS-safelisted, so a script cannot read them unless exposed.
		ExposeHeaders: []string{"ETag", "Retry-After", echo.HeaderXRequestID},
		MaxAge:        int((10 * time.Minute).Seconds()),
	}
}

// rateLimit allows rps requests per second per client IP, with bursts of
// twice that (a map pan fires several requests at once). The client IP
// comes from the server's IPExtractor: on the cluster Traefik preserves the
// source address (externalTrafficPolicy: Local) and replaces any
// X-Forwarded-For a client sends. Counters live in this pod's memory.
func rateLimit(rps float64) echo.MiddlewareFunc {
	return echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Store: echomw.NewRateLimiterMemoryStoreWithConfig(echomw.RateLimiterMemoryStoreConfig{
			Rate: rps, Burst: int(rps * 2), ExpiresIn: 3 * time.Minute,
		}),
		DenyHandler: func(c *echo.Context, _ string, _ error) error {
			c.Response().Header().Set("Retry-After", "1")
			return httpx.Abort(http.StatusTooManyRequests, "rate limit exceeded; retry after 1 s")
		},
	})
}

// requestLog logs each request at debug, and failures at info, skipping
// probes and scrapes.
func requestLog(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			err := next(c)
			path := c.Request().URL.Path
			if path == "/metrics" || path == "/health/live" || path == "/health/ready" {
				return err
			}
			status := httpx.StatusOf(c, err)
			level := slog.LevelDebug
			if status >= 400 {
				level = slog.LevelInfo
			}
			logger.Log(c.Request().Context(), level, "request",
				"method", c.Request().Method, "route", c.Path(), "status", status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", c.Response().Header().Get(echo.HeaderXRequestID))
			return err
		}
	}
}

// Check is one entry of the readiness report.
type Check struct {
	Status  string `json:"status"` // healthy | unhealthy
	Message string `json:"message,omitempty"`
}

// Health collects readiness checks.
type Health struct {
	mu     sync.Mutex
	checks map[string]func(ctx context.Context) Check
}

// NewHealth returns an empty registry.
func NewHealth() *Health { return &Health{checks: map[string]func(context.Context) Check{}} }

// Add registers or replaces a named readiness check.
func (h *Health) Add(name string, check func(ctx context.Context) Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

func (h *Health) run(ctx context.Context) map[string]Check {
	h.mu.Lock()
	names := make([]string, 0, len(h.checks))
	for n := range h.checks {
		names = append(names, n)
	}
	h.mu.Unlock()
	sort.Strings(names)
	out := make(map[string]Check, len(names))
	for _, n := range names {
		h.mu.Lock()
		check := h.checks[n]
		h.mu.Unlock()
		out[n] = check(ctx)
	}
	return out
}

// Pinger is anything that can prove a backend is reachable; *pgxpool.Pool
// is one.
type Pinger interface {
	Ping(ctx context.Context) error
}

// AddDatabaseCheck registers the database readiness check. A nil db (the
// routes command, DB-less tests) is always unhealthy.
func AddDatabaseCheck(h *Health, db Pinger, logger *slog.Logger) {
	h.Add("database", func(ctx context.Context) Check {
		if db == nil {
			return Check{Status: "unhealthy", Message: "database not configured"}
		}
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			logger.Error("readiness: database ping failed", "error", err)
			return Check{Status: "unhealthy", Message: "database unreachable"}
		}
		return Check{Status: "healthy"}
	})
}

type readiness struct {
	Status  string           `json:"status"` // ready | not_ready
	Checks  map[string]Check `json:"checks"`
	Version string           `json:"version"`
}

// LiveHandler answers the liveness probe.
func LiveHandler(c *echo.Context) error {
	return httpx.OK(c, map[string]string{"status": "ok"})
}

func registerOps(d *Deps, r *Routes) {
	// The norviq-app chart probes /health/live and /health/ready by default.
	r.Root.GET("/health/live", LiveHandler)
	r.Root.GET("/health/ready", func(c *echo.Context) error {
		checks := d.Health.run(c.Request().Context())
		status, code := "ready", http.StatusOK
		for _, ch := range checks {
			if ch.Status == "unhealthy" {
				status, code = "not_ready", http.StatusServiceUnavailable
				break
			}
		}
		return httpx.JSON(c, code, readiness{Status: status, Checks: checks, Version: d.Config.Version()})
	})
	r.Root.GET("/metrics", echo.WrapHandler(metrics.Handler()))
}
