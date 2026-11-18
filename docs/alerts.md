# Shield alerts (S1.9)

The rules below go into the infra repo at S3.9a, as a `"shield"` group in
`platform/infra/observability/alerts/platform_rules.py`. That file
provisions Grafana-managed alerts; the Prometheus YAML next to it is never
loaded. They are kept here until then because the infra repo deploys on
merge, and Shield is not deployed yet.

`job` values assume the release names from ARCHITECTURE §6:
`shield-worker-production` exports the source metrics and
`shield-api-production` the stream ones. Check them against the scrape
config when the apps go live.

## Metrics

| Metric | From | Meaning |
|---|---|---|
| `shield_source_last_success_timestamp_seconds{source}` | worker | Unix time of the last good poll. Seeded at worker start from `sources.last_success_at`, or with the start time if the source never succeeded |
| `shield_source_poll_interval_seconds{source}` | worker | the configured poll interval |
| `shield_source_lag_seconds{source}` | worker | our last fetch minus the source's own last update (`Last-Modified`; FIRMS uses its newest file). Exported for USGS, GDACS and FIRMS. EONET sends no `Last-Modified`, and Meteoalarm's adapter reports none across its 37 country feeds (revisit with MeteoGate access), so neither has a series |
| `shield_source_polls_total{source,result}` | worker | polls by `ok`, `not_modified` or `error` |
| `shield_event_publish_seconds` | api | event write (its transaction start) → stream fan-out |
| `shield_stream_subscribers`, `shield_stream_dropped_total` | api | open streams; slow clients dropped |

Measured on a live poll (2026-11-18): USGS 47 s, GDACS 594 s (2x its 5 min
interval), FIRMS 33 min (2.2x its 15 min interval). That is normal upstream
cadence, which is why lag does not alert at 3x our interval.

## Rules

```python
"shield": [
    dict(
        uid="shield-source-stale",
        title="ShieldSourceStale",
        ds="prom",
        # Our side: no successful poll for 3 intervals, the same threshold as
        # `stale` in /v1/sources (ADR 8, ADR 16).
        expr=(
            '(time() - max by (source) (shield_source_last_success_timestamp_seconds{job="shield-worker-production"}))'
            ' / max by (source) (shield_source_poll_interval_seconds{job="shield-worker-production"})'
        ),
        # Every source has a series from worker start (seeded from the
        # database, or the start time if it never succeeded), so NoData means
        # the worker itself is gone: alert on it rather than the default OK.
        op="gt", value=3, for_="5m", severity="warning", no_data="Alerting",
        summary="Shield source {{ $labels.source }} has not polled successfully for {{ humanize $values.A.Value }} intervals",
    ),
    dict(
        uid="shield-source-silent",
        title="ShieldSourceSilent",
        ds="prom",
        # Their side: the source itself stopped publishing. Upstream cadence,
        # not our interval, bounds normal lag (FIRMS: up to ~45 min on a 15 min
        # poll), so the threshold is absolute: 6 h.
        expr='max by (source) (shield_source_lag_seconds{job="shield-worker-production"})',
        op="gt", value=21600, for_="30m", severity="warning",
        summary="Shield source {{ $labels.source }} has not published for {{ humanizeDuration $values.A.Value }}",
    ),
    dict(
        uid="shield-publish-slow",
        title="ShieldEventPublishSlow",
        ds="prom",
        # ADR 16 budget: visible within one poll interval + 5 s. The fan-out
        # part should take well under 10 s.
        expr=(
            'histogram_quantile(0.95, sum by (le) (rate(shield_event_publish_seconds_bucket'
            '{job="shield-api-production"}[15m])))'
        ),
        op="gt", value=10, for_="15m", severity="warning",
        summary="Shield p95 write → stream fan-out is {{ humanize $values.A.Value }}s",
    ),
],
```
