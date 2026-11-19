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
