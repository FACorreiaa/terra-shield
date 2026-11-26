package severity

// Tropical cyclone severity from maximum sustained wind, used as a floor
// under GDACS's impact alert (owner, 2026-11-19): a storm's strength
// counts even over open ocean, as an earthquake's magnitude does. Bands
// follow the Saffir-Simpson scale in km/h:
//
//   - below hurricane force (< 119): minor; the alert alone decides
//   - Category 1–2 (119–177): moderate
//   - Category 3–4 (178–251): severe
//   - Category 5 (≥ 252): extreme
func CycloneWindKmh(kmh float64) Level {
	switch {
	case kmh >= 252:
		return Extreme
	case kmh >= 178:
		return Severe
	case kmh >= 119:
		return Moderate
	}
	return Minor
}

// CycloneWindKt is CycloneWindKmh for a wind in knots, the unit NHC and
// the Saffir-Simpson scale are defined in. Rating from knots directly
// keeps the band edges exact: 136 kt (Category 4) converts to 251.9 km/h,
// which rounds onto Category 5's 252.
//
//   - below 64 kt: minor
//   - Category 1–2 (64–95 kt): moderate
//   - Category 3–4 (96–136 kt): severe
//   - Category 5 (≥ 137 kt): extreme
func CycloneWindKt(kt float64) Level {
	switch {
	case kt >= 137:
		return Extreme
	case kt >= 96:
		return Severe
	case kt >= 64:
		return Moderate
	}
	return Minor
}
