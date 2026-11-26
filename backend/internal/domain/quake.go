package domain

import "time"

// QuakeOrigin is an earthquake as two networks would both report it, to
// tell when they did (ADR 34).
type QuakeOrigin struct {
	At       time.Time
	Lon, Lat float64
	Mag      float64
}

// QuakeMatch is how close two reports must be to be the same earthquake.
type QuakeMatch struct {
	Within   time.Duration // of the origin time
	Meters   float64       // between epicenters
	MagDelta float64       // between magnitudes, whatever their types
}
