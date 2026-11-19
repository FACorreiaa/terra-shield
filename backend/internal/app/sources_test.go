package app

import (
	"log/slog"
	"net/url"
	"testing"

	"shield/internal/config"
	"shield/internal/store"
)

// TestSourcesDescribeThemselves keeps the Sources screen (S3.7) complete:
// every adapter the worker schedules says what it covers and how often
// it publishes, and any terms page it names is an absolute https URL.
func TestSourcesDescribeThemselves(t *testing.T) {
	c := &container{cfg: config.Base{}, logger: slog.New(slog.DiscardHandler), repos: store.New(nil)}
	seen := map[string]bool{}
	for _, s := range c.sources() {
		info := s.Info()
		if seen[info.ID] {
			t.Errorf("source %s registered twice", info.ID)
		}
		seen[info.ID] = true
		if info.Scope == "" || info.Cadence == "" {
			t.Errorf("%s: scope %q, cadence %q; both are required", info.ID, info.Scope, info.Cadence)
		}
		if info.TermsURL == "" {
			continue // unverified terms page: omitted, never guessed
		}
		if u, err := url.Parse(info.TermsURL); err != nil || u.Scheme != "https" || u.Host == "" {
			t.Errorf("%s: terms URL %q is not an absolute https URL", info.ID, info.TermsURL)
		}
	}
}
