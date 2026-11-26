// Package shapefile reads the parts of zipped ESRI shapefiles Shield uses:
// polygon geometry and character attributes. The NWS zone files and NHC's
// forecast cones both come this way.
package shapefile

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
)

// Member reads the archive's one file whose name ends with suffix
// ("_pgn.shp", or ".dbf" when there is a single layer), case-insensitively.
func Member(zr *zip.Reader, suffix string) ([]byte, error) {
	suffix = strings.ToLower(suffix)
	i := slices.IndexFunc(zr.File, func(f *zip.File) bool { return strings.HasSuffix(strings.ToLower(f.Name), suffix) })
	if i < 0 {
		return nil, fmt.Errorf("shapefile: no %s in the archive", suffix)
	}
	rc, err := zr.File[i].Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 256<<20))
}

const (
	header              = 100
	shapeNull    uint32 = 0
	shapePolygon uint32 = 5
)

// Polygons returns each record's rings, in file order; a null shape has
// none. Coordinates are as stored: callers check their range (NHC's cones
// run past 180° of longitude when they cross the antimeridian).
func Polygons(b []byte) ([][][][2]float64, error) {
	if len(b) < header || binary.BigEndian.Uint32(b[0:4]) != 9994 {
		return nil, errors.New("shapefile: not a shapefile")
	}
	if t := binary.LittleEndian.Uint32(b[32:36]); t != shapePolygon {
		return nil, fmt.Errorf("shapefile: shape type %d, want polygons", t)
	}
	var out [][][][2]float64
	for off := header; off < len(b); {
		if off+8 > len(b) {
			return nil, errors.New("shapefile: truncated record header")
		}
		n := int(binary.BigEndian.Uint32(b[off+4:off+8])) * 2 // 16-bit words
		rec := b[off+8:]
		if n < 4 || n > len(rec) {
			return nil, fmt.Errorf("shapefile: record %d: bad length", len(out)+1)
		}
		rings, err := polygon(rec[:n])
		if err != nil {
			return nil, fmt.Errorf("shapefile: record %d: %w", len(out)+1, err)
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
			if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || y < -90 || y > 90 || x < -360 || x > 360 {
				return nil, fmt.Errorf("ring %d: point %g,%g out of range", i, x, y)
			}
			ring = append(ring, [2]float64{x, y})
		}
		rings = append(rings, ring)
	}
	return rings, nil
}

// DBF returns the named fields of every record, trimmed; a record marked
// deleted is nil, so rows stay aligned with shapes. Every named field must
// exist. Text is read as Latin-1's ASCII subset, which is what NWS and
// NHC write.
func DBF(b []byte, names ...string) ([]map[string]string, error) {
	if len(b) < 33 {
		return nil, errors.New("shapefile: truncated dbf")
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
			return nil, fmt.Errorf("shapefile: dbf has no %s field", name)
		}
	}
	if pos != recLen || headerLen+n*recLen > len(b) {
		return nil, errors.New("shapefile: dbf record layout does not add up")
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
