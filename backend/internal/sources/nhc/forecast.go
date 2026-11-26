package nhc

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"shield/internal/geo/shapefile"
)

// forecast is one forecast advisory's cone and track points, read from
// NHC's "5day" GIS archive (al052025_5day_020.zip). It holds the polygon
// layer (_pgn: the cone) and the points layer (_pts: one row per forecast
// hour, with position and intensity); the line layer is the points
// joined, so it is not read.
type forecast struct {
	Cone   json.RawMessage // GeoJSON MultiPolygon, split at the antimeridian
	Points []point
}

// point is one forecast position. Wind and gust are knots; zero pressure
// is unknown (NHC forecasts pressure only for the initial position).
type point struct {
	Tau      int       // hours after the advisory's synoptic time
	At       time.Time // zero when the valid time is unreadable
	Lon, Lat float64   // lon in [-180, 180]
	WindKt   float64
	GustKt   float64
	Pressure float64 // mb
	Type     string  // NHC storm type at that hour (TD, TS, HU, MH, PTC, ...)
}

// missing is NHC's sentinel for no value (9999 in MSLP, TCDIR, TCSPD).
const missing = 9999

// parseForecast reads the zip. issued is the advisory's issuance, which
// anchors the points' day-and-hour valid times to a month and year.
func parseForecast(zipped []byte, issued time.Time) (*forecast, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil {
		return nil, fmt.Errorf("nhc forecast: %w", err)
	}
	shp, err := shapefile.Member(zr, "_5day_pgn.shp")
	if err != nil {
		return nil, fmt.Errorf("nhc forecast: %w", err)
	}
	dbf, err := shapefile.Member(zr, "_5day_pts.dbf")
	if err != nil {
		return nil, fmt.Errorf("nhc forecast: %w", err)
	}
	records, err := shapefile.Polygons(shp)
	if err != nil {
		return nil, fmt.Errorf("nhc forecast: %w", err)
	}
	cone, err := coneGeoJSON(records)
	if err != nil {
		return nil, err
	}
	rows, err := shapefile.DBF(dbf, "TAU", "LAT", "LON", "MAXWIND", "GUST", "MSLP", "VALIDTIME", "STORMTYPE")
	if err != nil {
		return nil, fmt.Errorf("nhc forecast: %w", err)
	}
	f := &forecast{Cone: cone}
	for i, row := range rows {
		if row == nil {
			continue
		}
		p, err := parsePoint(row, issued)
		if err != nil {
			return nil, fmt.Errorf("nhc forecast: point %d: %w", i+1, err)
		}
		f.Points = append(f.Points, p)
	}
	if len(f.Points) == 0 {
		return nil, errors.New("nhc forecast: no track points")
	}
	return f, nil
}

func parsePoint(row map[string]string, issued time.Time) (point, error) {
	num := func(name string) (float64, error) {
		v, err := strconv.ParseFloat(row[name], 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("%s %q", name, row[name])
		}
		return v, nil
	}
	tau, err := num("TAU")
	if err != nil || tau < 0 || tau > 240 {
		return point{}, fmt.Errorf("TAU %q", row["TAU"])
	}
	lat, err := num("LAT")
	if err != nil || lat < -90 || lat > 90 {
		return point{}, fmt.Errorf("LAT %q", row["LAT"])
	}
	lon, err := num("LON")
	if err != nil || lon < -540 || lon > 540 {
		return point{}, fmt.Errorf("LON %q", row["LON"])
	}
	p := point{Tau: int(tau), Lat: lat, Lon: wrapLon(lon), Type: row["STORMTYPE"]}
	// Intensity may be missing (9999) or blank; the position is enough.
	p.WindKt, _ = present(num("MAXWIND"))
	p.GustKt, _ = present(num("GUST"))
	p.Pressure, _ = present(num("MSLP"))
	p.At = validTime(row["VALIDTIME"], issued.Add(time.Duration(tau)*time.Hour))
	return p, nil
}

// present drops NHC's missing sentinel, blanks and nonsense.
func present(v float64, err error) (float64, bool) {
	if err != nil || v <= 0 || v >= missing {
		return 0, false
	}
	return v, true
}

// validTime reads NHC's "DD/HHMM" (UTC day of month, hour and minute) as
// the date nearest approx, the advisory's issuance plus the forecast
// hour: forecasts run past month and year ends. NHC's synoptic time
// precedes issuance by about three hours. Zero when unreadable or not
// within a day of approx.
func validTime(s string, approx time.Time) time.Time {
	approx = approx.UTC()
	dayS, hm, ok := strings.Cut(strings.TrimSpace(s), "/")
	if !ok || len(hm) != 4 {
		return time.Time{}
	}
	day, err1 := strconv.Atoi(dayS)
	hour, err2 := strconv.Atoi(hm[:2])
	minute, err3 := strconv.Atoi(hm[2:])
	if err1 != nil || err2 != nil || err3 != nil || day < 1 || day > 31 || hour > 23 || minute > 59 {
		return time.Time{}
	}
	var best time.Time
	for _, dm := range []int{-1, 0, 1} {
		first := time.Date(approx.Year(), approx.Month()+time.Month(dm), 1, 0, 0, 0, 0, time.UTC)
		t := time.Date(first.Year(), first.Month(), day, hour, minute, 0, 0, time.UTC)
		if t.Month() != first.Month() {
			continue // the 31st of a 30-day month
		}
		if best.IsZero() || t.Sub(approx).Abs() < best.Sub(approx).Abs() {
			best = t
		}
	}
	if best.IsZero() || best.Sub(approx).Abs() > 24*time.Hour {
		return time.Time{}
	}
	return best
}

// wrapLon brings a longitude into [-180, 180]. NHC writes Central and
// West Pacific positions past the antimeridian as -192.7 and their cones
// as 198.4.
func wrapLon(x float64) float64 {
	for x > 180 {
		x -= 360
	}
	for x < -180 {
		x += 360
	}
	return x
}

// coneGeoJSON makes the polygon records one MultiPolygon. The shapefile
// spec winds outer rings clockwise, but NHC's cones are counter-clockwise,
// so orientation is read relative to each record's first ring, which is
// always an outer: a later ring wound the same way is another outer, one
// wound the other way a hole in the outer before it. A cone that crosses
// the antimeridian is cut there and its far side shifted by 360°, so
// every part lies within [-180, 180].
func coneGeoJSON(records [][][][2]float64) (json.RawMessage, error) {
	var polys [][][][2]float64
	for _, rings := range records {
		var first float64 // signed area of the record's first ring
		for i, ring := range rings {
			a := signedArea(ring)
			if i == 0 {
				first = a
			}
			if i == 0 || (a < 0) == (first < 0) {
				polys = append(polys, [][][2]float64{ring})
				continue
			}
			polys[len(polys)-1] = append(polys[len(polys)-1], ring)
		}
	}
	var out [][][][2]float64
	for _, poly := range polys {
		out = append(out, splitAntimeridian(poly)...)
	}
	if len(out) == 0 {
		return nil, errors.New("nhc forecast: empty cone")
	}
	return json.Marshal(map[string]any{"type": "MultiPolygon", "coordinates": out})
}

// signedArea is the shoelace sum: negative for a clockwise ring.
func signedArea(ring [][2]float64) float64 {
	var a float64
	for i := 0; i+1 < len(ring); i++ {
		a += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}
	return a / 2
}

// splitAntimeridian cuts a polygon into the parts that fall in each
// 360°-wide band of longitude, each shifted into [-180, 180]. A polygon
// already within range is returned as is. Holes are cut the same way and
// kept with the part they fall in; the store repairs any sliver the cut
// leaves along the meridian.
func splitAntimeridian(poly [][][2]float64) [][][][2]float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range poly[0] {
		lo, hi = math.Min(lo, p[0]), math.Max(hi, p[0])
	}
	if lo >= -180 && hi <= 180 {
		return [][][][2]float64{poly}
	}
	var out [][][][2]float64
	for shift := -720.0; shift <= 720; shift += 360 {
		left, right := -180+shift, 180+shift
		if hi <= left || lo >= right {
			continue
		}
		var part [][][2]float64
		for i, ring := range poly {
			r := clipX(clipX(ring, left, true), right, false)
			if len(r) < 4 || math.Abs(signedArea(r)) < 1e-9 {
				if i == 0 {
					break // no outer ring on this side
				}
				continue
			}
			for j := range r {
				r[j][0] -= shift
			}
			part = append(part, r)
		}
		if len(part) > 0 {
			out = append(out, part)
		}
	}
	return out
}

// clipX keeps the part of a closed ring on one side of the meridian x
// (x and east when keepAbove, else x and west): Sutherland–Hodgman
// against one half-plane. The result is closed, or nil.
func clipX(ring [][2]float64, x float64, keepAbove bool) [][2]float64 {
	if len(ring) < 4 {
		return nil
	}
	inside := func(p [2]float64) bool {
		if keepAbove {
			return p[0] >= x
		}
		return p[0] <= x
	}
	cross := func(a, b [2]float64) [2]float64 {
		t := (x - a[0]) / (b[0] - a[0])
		return [2]float64{x, a[1] + t*(b[1]-a[1])}
	}
	pts := ring[:len(ring)-1] // drop the closing point
	var out [][2]float64
	for i, cur := range pts {
		prev := pts[(i+len(pts)-1)%len(pts)]
		switch {
		case inside(cur) && inside(prev):
			out = append(out, cur)
		case inside(cur):
			out = append(out, cross(prev, cur), cur)
		case inside(prev):
			out = append(out, cross(prev, cur))
		}
	}
	if len(out) < 3 {
		return nil
	}
	return append(out, out[0])
}
