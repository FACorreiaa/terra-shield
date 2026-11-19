// Package nwszones reads the National Weather Service's public forecast
// zones: the shapes behind the UGC zone codes (HIZ001, GUZ001, …) that NWS
// products such as tsunami warnings name instead of drawing. NWS publishes
// them as a dated shapefile, public domain, pinned here.
//
// Only what Shield needs of the format is read: polygon shapes (type 5)
// and the dBASE attribute table. The rings are handed to PostGIS as a
// MultiLineString, and ST_BuildArea there sorts them into polygons and
// holes, so the reader does not need to.
package nwszones

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"

	"shield/internal/domain"
	"shield/internal/geo/dataset"
)

// The 16 April 2026 release (z_16ap26): 4,157 records, 4,080 zones, every
// state and territory including Guam, American Samoa, the Northern
// Marianas, Puerto Rico and the Virgin Islands. NWS replaces the file when
// zones change (a few times a year); moving to a new one means updating
// the pin, and `shield migrate` reloads it.
var Zones = dataset.Pin{
	Name:     "nws_zones",
	URL:      "https://www.weather.gov/source/gis/Shapefiles/WSOM/z_16ap26.zip",
	SHA256:   "010dc5eab64b6e326c969e252bd016f56a037b0444df8303658707bca4cc7fd8",
	MaxBytes: 96 << 20, // 26.5 MB today
}

// Processing is how zones are prepared on load, recorded with the pin so
// a change reloads them. 0.001° is about 100 m, as for MeteoAlarm areas.
const Processing = "simplify-0.001"

// SimplifyDegrees is the tolerance Processing names.
const SimplifyDegrees = 0.001

// Version is what geo_datasets records for the pin loaded with Processing.
func Version(p dataset.Pin) string { return p.SHA256 + "+" + Processing }

var (
	stateRe = regexp.MustCompile(`^[A-Z]{2}$`)
	zoneRe  = regexp.MustCompile(`^[0-9]{3}$`)
)

// Parse reads the zipped shapefile. A zone spread over several records
// (77 in the 2026 file) becomes one zone with all their rings. Any
// malformed record fails the whole file: a half-loaded zone table would
// draw warnings smaller than they are.
func Parse(zipped []byte) ([]domain.ForecastZone, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil {
		return nil, fmt.Errorf("nws zones: %w", err)
	}
	shp, err := member(zr, ".shp")
	if err != nil {
		return nil, err
	}
	dbf, err := member(zr, ".dbf")
	if err != nil {
		return nil, err
	}
	shapes, err := readPolygons(shp)
	if err != nil {
		return nil, err
	}
	rows, err := readDBF(dbf, "STATE", "ZONE", "NAME")
	if err != nil {
		return nil, err
	}
	if len(rows) != len(shapes) {
		return nil, fmt.Errorf("nws zones: %d attribute rows for %d shapes", len(rows), len(shapes))
	}

	type acc struct {
		state, name string
		rings       [][][2]float64
	}
	byUGC := map[string]*acc{}
	for i, row := range rows {
		if row == nil {
			continue // deleted in the attribute table
		}
		state, zone, name := row["STATE"], row["ZONE"], row["NAME"]
		if !stateRe.MatchString(state) || !zoneRe.MatchString(zone) {
			return nil, fmt.Errorf("nws zones: record %d: bad code %q/%q", i+1, state, zone)
		}
		if len(shapes[i]) == 0 {
			continue // a null shape draws nothing
		}
		ugc := state + "Z" + zone
		a := byUGC[ugc]
		if a == nil {
			a = &acc{state: state, name: name}
			byUGC[ugc] = a
		}
		a.rings = append(a.rings, shapes[i]...)
	}
	if len(byUGC) == 0 {
		return nil, errors.New("nws zones: no zones")
	}
	out := make([]domain.ForecastZone, 0, len(byUGC))
	for ugc, a := range byUGC {
		geom, err := json.Marshal(map[string]any{"type": "MultiLineString", "coordinates": a.rings})
		if err != nil {
			return nil, err
		}
		out = append(out, domain.ForecastZone{UGC: ugc, State: a.state, Name: a.name, Rings: geom})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UGC < out[j].UGC })
	return out, nil
}

func member(zr *zip.Reader, ext string) ([]byte, error) {
	i := slices.IndexFunc(zr.File, func(f *zip.File) bool { return strings.HasSuffix(strings.ToLower(f.Name), ext) })
	if i < 0 {
		return nil, fmt.Errorf("nws zones: no %s in the archive", ext)
	}
	rc, err := zr.File[i].Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 256<<20))
}

const (
	shpHeader           = 100
	shapeNull    uint32 = 0
	shapePolygon uint32 = 5
)

// readPolygons returns each record's rings, in file order; a null shape
// has none.
func readPolygons(b []byte) ([][][][2]float64, error) {
	if len(b) < shpHeader || binary.BigEndian.Uint32(b[0:4]) != 9994 {
		return nil, errors.New("nws zones: not a shapefile")
	}
	if t := binary.LittleEndian.Uint32(b[32:36]); t != shapePolygon {
		return nil, fmt.Errorf("nws zones: shape type %d, want polygons", t)
	}
	var out [][][][2]float64
	for off := shpHeader; off < len(b); {
		if off+8 > len(b) {
			return nil, errors.New("nws zones: truncated record header")
		}
		n := int(binary.BigEndian.Uint32(b[off+4:off+8])) * 2 // 16-bit words
		rec := b[off+8:]
		if n < 4 || n > len(rec) {
			return nil, fmt.Errorf("nws zones: record %d: bad length", len(out)+1)
		}
		rings, err := polygon(rec[:n])
		if err != nil {
			return nil, fmt.Errorf("nws zones: record %d: %w", len(out)+1, err)
		}
		out = append(out, rings)
		off += 8 + n
	}
	return out, nil
}

func polygon(c []byte) ([][][2]float64, error) {
	switch t := binary.LittleEndian.Uint32(c[0:4]); t {
	case shapeNull:
		return nil, nil
	case shapePolygon:
	default:
		return nil, fmt.Errorf("shape type %d", t)
	}
	if len(c) < 44 {
		return nil, errors.New("truncated polygon")
	}
	parts := int(binary.LittleEndian.Uint32(c[36:40]))
	points := int(binary.LittleEndian.Uint32(c[40:44]))
	if parts < 1 || points < 4 || 44+4*parts+16*points > len(c) {
		return nil, fmt.Errorf("%d parts, %d points in %d bytes", parts, points, len(c))
	}
	starts := make([]int, parts+1)
	for i := range parts {
		starts[i] = int(binary.LittleEndian.Uint32(c[44+4*i:]))
	}
	starts[parts] = points
	base := 44 + 4*parts
	rings := make([][][2]float64, 0, parts)
	for i := range parts {
		from, to := starts[i], starts[i+1]
		if from < 0 || to > points || to-from < 4 {
			return nil, fmt.Errorf("ring %d: points %d..%d", i, from, to)
		}
		ring := make([][2]float64, 0, to-from)
		for p := from; p < to; p++ {
			x := math.Float64frombits(binary.LittleEndian.Uint64(c[base+16*p:]))
			y := math.Float64frombits(binary.LittleEndian.Uint64(c[base+16*p+8:]))
			if x < -180 || x > 180 || y < -90 || y > 90 || math.IsNaN(x) || math.IsNaN(y) {
				return nil, fmt.Errorf("ring %d: point %g,%g out of range", i, x, y)
			}
			ring = append(ring, [2]float64{x, y})
		}
		rings = append(rings, ring)
	}
	return rings, nil
}

// readDBF returns the named character fields of every record, trimmed;
// a record marked deleted is nil, so rows stay aligned with shapes. The
// file is ASCII.
func readDBF(b []byte, names ...string) ([]map[string]string, error) {
	if len(b) < 33 {
		return nil, errors.New("nws zones: truncated dbf")
	}
	n := int(binary.LittleEndian.Uint32(b[4:8]))
	headerLen := int(binary.LittleEndian.Uint16(b[8:10]))
	recLen := int(binary.LittleEndian.Uint16(b[10:12]))
	type field struct{ off, len int }
	fields := map[string]field{}
	pos := 1 // after the deletion flag
	for off := 32; off+32 <= len(b) && b[off] != 0x0D; off += 32 {
		name := string(bytes.TrimRight(b[off:off+11], "\x00"))
		l := int(b[off+16])
		fields[name] = field{pos, l}
		pos += l
	}
	for _, name := range names {
		if _, ok := fields[name]; !ok {
			return nil, fmt.Errorf("nws zones: dbf has no %s field", name)
		}
	}
	if pos != recLen || headerLen+n*recLen > len(b) {
		return nil, errors.New("nws zones: dbf record layout does not add up")
	}
	out := make([]map[string]string, 0, n)
	for i := range n {
		rec := b[headerLen+i*recLen : headerLen+(i+1)*recLen]
		if rec[0] == '*' {
			out = append(out, nil)
			continue
		}
		row := make(map[string]string, len(names))
		for _, name := range names {
			f := fields[name]
			row[name] = strings.TrimSpace(string(rec[f.off : f.off+f.len]))
		}
		out = append(out, row)
	}
	return out, nil
}
