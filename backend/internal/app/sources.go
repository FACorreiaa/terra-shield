package app

import (
	"time"

	"shield/internal/fetch"
	"shield/internal/ingest"
	"shield/internal/sources/eonet"
	"shield/internal/sources/firms"
	"shield/internal/sources/gdacs"
	"shield/internal/sources/meteoalarm"
	"shield/internal/sources/tsunami"
	"shield/internal/sources/usgs"
)

// sources is every feed adapter the worker schedules, wired to its
// repository and HTTP client.
func (c *container) sources() []ingest.Source {
	// One client for the API-style feeds, so the per-host limiter spans
	// them. The User-Agent names the product only, nothing personal.
	feeds := fetch.New(fetch.Options{UserAgent: c.userAgent()})
	// FIRMS' global CSVs are 6–7 MB each today and grow in fire season.
	bulk := fetch.New(fetch.Options{UserAgent: c.userAgent(), MaxBody: 64 << 20, Timeout: 2 * time.Minute})

	return []ingest.Source{
		usgs.New(feeds, c.repos.Events),
		gdacs.New(feeds, c.repos.Events, c.logger),
		eonet.New(feeds, c.repos.Events),
		firms.New(bulk, c.repos.Fires),
		meteoalarm.New(feeds, c.repos.Events, meteoalarm.NewResolver(c.repos.WarningAreas), c.logger),
		tsunami.New(feeds, c.repos.Events, c.repos.NWSZones, c.logger),
	}
}

func (c *container) userAgent() string { return "Shield-HazardMap/" + c.cfg.Version() }
