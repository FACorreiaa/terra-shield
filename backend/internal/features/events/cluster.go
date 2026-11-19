package events

import (
	"math"

	"shield/internal/domain"
	"shield/internal/severity"
)

// clusterCellPx is the clustering grid, in screen pixels: events closer
// than about a marker's width would overlap.
const clusterCellPx = 64

// cluster groups events whose centroids share a cell of a pixel grid laid
// over the Web Mercator map at zoom (ADR 9). A cell holding one event
// leaves it as is. Input order (most severe first) is kept for both the
// clusters, by their first member, and the remaining events.
func cluster(recs []domain.EventRecord, zoom int) ([]Cluster, []domain.EventRecord) {
	type cell struct{ x, y int64 }
	members := map[cell][]int{}
	var order []cell
	for i := range recs {
		x, y := mercatorPx(recs[i].Longitude, recs[i].Latitude, zoom)
		k := cell{int64(math.Floor(x / clusterCellPx)), int64(math.Floor(y / clusterCellPx))}
		if _, seen := members[k]; !seen {
			order = append(order, k)
		}
		members[k] = append(members[k], i)
	}

	var clusters []Cluster
	var singles []domain.EventRecord
	for _, k := range order {
		idx := members[k]
		if len(idx) == 1 {
			singles = append(singles, recs[idx[0]])
			continue
		}
		clusters = append(clusters, merge(recs, idx))
	}
	return clusters, singles
}

func merge(recs []domain.EventRecord, idx []int) Cluster {
	c := Cluster{
		Count: len(idx), Types: map[string]int{}, Severities: map[string]int{},
		Statuses: map[string]int{}, ActiveSeverities: map[string]int{},
	}
	maxLevel := severity.Minor
	var sumLat, sumLon float64
	c.BBox = [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, i := range idx {
		r := &recs[i]
		sumLat += r.Latitude
		sumLon += r.Longitude
		c.Types[string(r.Type)]++
		c.Severities[r.Severity]++
		c.Statuses[string(r.Status)]++
		if r.Status == domain.StatusActive {
			c.ActiveSeverities[r.Severity]++
		}
		if l, ok := severity.Parse(r.Severity); ok && l > maxLevel {
			maxLevel = l
		}
		c.BBox[0] = math.Min(c.BBox[0], r.Longitude)
		c.BBox[1] = math.Min(c.BBox[1], r.Latitude)
		c.BBox[2] = math.Max(c.BBox[2], r.Longitude)
		c.BBox[3] = math.Max(c.BBox[3], r.Latitude)
	}
	n := float64(len(idx))
	c.Centroid = Coordinate{Latitude: sumLat / n, Longitude: sumLon / n}
	c.MaxSeverity = maxLevel.String()
	return c
}

// maxMercatorLat is where Web Mercator is cut off (a square world).
const maxMercatorLat = 85.05112878

// mercatorPx converts lon/lat to pixel coordinates of a 256 px tile
// pyramid at zoom.
func mercatorPx(lon, lat float64, zoom int) (float64, float64) {
	lat = math.Max(-maxMercatorLat, math.Min(maxMercatorLat, lat))
	size := 256 * math.Exp2(float64(zoom))
	x := (lon + 180) / 360 * size
	sin := math.Sin(lat * math.Pi / 180)
	y := (0.5 - math.Log((1+sin)/(1-sin))/(4*math.Pi)) * size
	return x, y
}
