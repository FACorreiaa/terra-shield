package domain

import (
	"fmt"
	"time"
)

// Aircraft is one live position on the flights layer (L1). Positions are
// never stored: they are served from a short in-memory cache and dropped.
type Aircraft struct {
	// Hex is the 24-bit ICAO address; a leading "~" marks a non-ICAO one.
	Hex          string
	Callsign     string // trimmed; empty when not reported
	Registration string // from the feed's aircraft database, when known
	TypeCode     string // ICAO type designator, when known
	Lon, Lat     float64
	// AltitudeFt is barometric altitude; nil when not reported or on the
	// ground.
	AltitudeFt      *int
	OnGround        bool
	GroundSpeedKt   *float64
	TrackDeg        *float64
	VerticalRateFpm *int
	Squawk          string
	// Emergency is the ADS-B emergency status, empty for "none".
	Emergency string
	// Military is the feed database's military flag, shown as the feed
	// gives it (owner, 2026-11-19).
	Military bool
	// PositionAt is when the position was received.
	PositionAt time.Time
}

// AircraftArea is one upstream answer for a circle.
type AircraftArea struct {
	Aircraft []Aircraft
	// At is when the feed generated the answer.
	At time.Time
	// QuotaRemaining is the feed's remaining request quota; -1 when the
	// feed did not say.
	QuotaRemaining int
}

// QuotaError is a feed refusing a request for quota: no request until
// RetryAfter has passed (zero when the feed did not say).
type QuotaError struct {
	RetryAfter time.Duration
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("feed quota exhausted (retry after %s)", e.RetryAfter)
}
