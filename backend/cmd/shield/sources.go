package main

import "shield/internal/ingest"

// registeredSources is every feed adapter the worker schedules. M1 adds
// USGS, GDACS, EONET, Meteoalarm and FIRMS here (internal/sources/<name>).
func registeredSources() []ingest.Source {
	return nil
}
