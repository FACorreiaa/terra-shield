package adsblol

import (
	_ "embed"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// recorded is a trimmed real /v2/point answer around Lisbon (38.5, -9.0),
// 2026-11-19: airborne, on-ground, military, LADD, no-position and
// stale-position aircraft.
//
//go:embed testdata/point.json
var recorded []byte

const recordedLat, recordedLon = 38.5, -9.0

// Replay serves the recorded answer for any /v2/point/{lat}/{lon}/{nm}
// request, moved to the requested center and stamped now, so client work
// (AIRCRAFT_FEED_URL) shows aircraft anywhere without spending the feed's
// quota. Development only: cmd/aircraft-replay.
func Replay(now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 5 || parts[0] != "v2" || parts[1] != "point" {
			http.NotFound(w, r)
			return
		}
		lat, err1 := strconv.ParseFloat(parts[2], 64)
		lon, err2 := strconv.ParseFloat(parts[3], 64)
		if err1 != nil || err2 != nil {
			http.Error(w, "bad point", http.StatusUnprocessableEntity)
			return
		}
		var env map[string]any
		if err := json.Unmarshal(recorded, &env); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ms := now().UnixMilli()
		env["now"], env["ctime"] = ms, ms
		acs, _ := env["ac"].([]any)
		for _, a := range acs {
			ac, _ := a.(map[string]any)
			if la, ok := ac["lat"].(float64); ok {
				ac["lat"] = math.Max(-90, math.Min(90, la-recordedLat+lat))
			}
			if lo, ok := ac["lon"].(float64); ok {
				ac["lon"] = math.Remainder(lo-recordedLon+lon, 360)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "500")
		_ = json.NewEncoder(w).Encode(env)
	})
}
