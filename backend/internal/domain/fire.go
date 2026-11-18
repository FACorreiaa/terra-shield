package domain

import "time"

// FireDetection is one satellite thermal anomaly (ADR 10): a detection,
// never by itself a "wildfire".
type FireDetection struct {
	Satellite  string // "NOAA-20", "NOAA-21", "Suomi NPP"
	AcquiredAt time.Time
	Lat, Lon   float64
	FRP        *float32 // fire radiative power, MW
	Confidence string   // VIIRS: low | nominal | high
	ISO2       string   // set by the store on insert; "" over sea
}

// FireQuery filters detections.
type FireQuery struct {
	BBox  *BBox
	Since time.Time
}

// FireCell is the detections in one grid cell, for a heatmap.
type FireCell struct {
	Lat, Lon float64 // cell center
	Count    int
	MaxFRP   *float32
	LatestAt time.Time
}
