// Package metrics holds Shield's Prometheus series.
//
// Grafana Cloud's free tier allows about 10k active series for the whole
// cluster, so labels stay low-cardinality: route templates, status classes
// and source IDs, never event IDs, countries or raw paths.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"shield/internal/httpx"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "shield_http_requests_total",
		Help: "HTTP requests by route template, method and status class.",
	}, []string{"route", "method", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "shield_http_request_duration_seconds",
		Help:    "HTTP request latency by route template.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"route"})

	// SourcePolls counts polls per source by result: ok, not_modified, error.
	SourcePolls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "shield_source_polls_total",
		Help: "Feed polls by source and result.",
	}, []string{"source", "result"})

	// SourceLastSuccess is the Unix time of each source's last good poll;
	// the staleness alert compares it with the poll interval.
	SourceLastSuccess = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "shield_source_last_success_timestamp_seconds",
		Help: "Unix time of the last successful poll per source.",
	}, []string{"source"})

	// SourceEventsUpserted counts events written per source.
	SourceEventsUpserted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "shield_source_events_upserted_total",
		Help: "Events inserted or updated per source.",
	}, []string{"source"})

	// StreamSubscribers is the number of open /v1/stream connections.
	StreamSubscribers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "shield_stream_subscribers",
		Help: "Open live-stream connections on this pod.",
	})

	// StreamDropped counts stream clients cut off for reading too slowly.
	StreamDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "shield_stream_dropped_total",
		Help: "Live-stream clients disconnected because their buffer filled.",
	})
)

// Handler serves the default registry.
func Handler() http.Handler { return promhttp.Handler() }

// StreamRoute is the live stream, excluded from request latency.
const StreamRoute = "/v1/stream"

// HTTPMiddleware records request counts and latency. Unmatched requests are
// labeled "unmatched" so scanners cannot grow the series count.
func HTTPMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			err := next(c)
			route := c.Path()
			if route == "" {
				route = "unmatched"
			}
			status := httpx.StatusOf(c, err)
			httpRequests.WithLabelValues(route, c.Request().Method, strconv.Itoa(status/100)+"xx").Inc()
			if route != StreamRoute { // a stream lasts minutes; it would swamp the latency buckets
				httpDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
			}
			return err
		}
	}
}
