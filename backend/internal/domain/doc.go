// Package domain holds Shield's core types: events, sources, fire
// detections and countries. It depends on nothing else in the module, so
// adapters, repositories, the ingest worker and the HTTP features can all
// share it without import cycles.
package domain
