package nwszones

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type rec struct {
	state, zone, name string
	rings             [][][2]float64 // nil: a null shape
}

// shapefile writes a zipped polygon shapefile with a STATE/ZONE/NAME dbf;
// deleted marks dbf rows flagged as deleted.
func shapefile(t *testing.T, recs []rec, deleted map[int]bool) []byte {
	t.Helper()
	var shp bytes.Buffer
	shp.Write(make([]byte, 100))
	for i, r := range recs {
		var c bytes.Buffer
		le := func(v any) { _ = binary.Write(&c, binary.LittleEndian, v) }
		if r.rings == nil {
			le(int32(0))
		} else {
			le(int32(5))
			le([4]float64{})
			n := 0
			for _, ring := range r.rings {
				n += len(ring)
			}
			le(int32(len(r.rings)))
			le(int32(n))
			start := 0
			for _, ring := range r.rings {
				le(int32(start))
				start += len(ring)
			}
			for _, ring := range r.rings {
				for _, p := range ring {
					le(p)
				}
			}
		}
		_ = binary.Write(&shp, binary.BigEndian, int32(i+1))
		_ = binary.Write(&shp, binary.BigEndian, int32(c.Len()/2))
		shp.Write(c.Bytes())
	}
	b := shp.Bytes()
	binary.BigEndian.PutUint32(b[0:4], 9994)
	binary.BigEndian.PutUint32(b[24:28], uint32(len(b)/2))
	binary.LittleEndian.PutUint32(b[28:32], 1000)
	binary.LittleEndian.PutUint32(b[32:36], 5)

	fields := []struct {
		name string
		l    int
	}{{"STATE", 2}, {"ZONE", 3}, {"NAME", 20}}
	var dbf bytes.Buffer
	hdr := make([]byte, 32)
	hdr[0] = 3
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(recs)))
	binary.LittleEndian.PutUint16(hdr[8:10], uint16(32+32*len(fields)+1))
	binary.LittleEndian.PutUint16(hdr[10:12], uint16(1+2+3+20))
	dbf.Write(hdr)
	for _, f := range fields {
		d := make([]byte, 32)
		copy(d, f.name)
		d[11] = 'C'
		d[16] = byte(f.l)
		dbf.Write(d)
	}
	dbf.WriteByte(0x0D)
	for i, r := range recs {
		flag := " "
		if deleted[i] {
			flag = "*"
		}
		dbf.WriteString(flag + r.state + r.zone + r.name + strings.Repeat(" ", 20-len(r.name)))
	}

	var z bytes.Buffer
	zw := zip.NewWriter(&z)
	for name, body := range map[string][]byte{"z.shp": b, "z.dbf": dbf.Bytes()} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(body)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return z.Bytes()
}

func square(x, y, s float64) [][2]float64 {
	return [][2]float64{{x, y}, {x + s, y}, {x + s, y + s}, {x, y + s}, {x, y}}
}

func TestParse(t *testing.T) {
	zipped := shapefile(t, []rec{
		{"HI", "001", "Niihau", [][][2]float64{square(-160.3, 21.7, 0.3)}},
		// One zone over two records, the first with a hole: all rings kept.
		{"GU", "001", "Guam", [][][2]float64{square(144.6, 13.2, 0.4), square(144.7, 13.3, 0.1)}},
		{"GU", "001", "Guam", [][][2]float64{square(145, 13, 0.05)}},
		{"AS", "001", "Tutuila", nil},                          // null shape: no zone
		{"XX", "999", "Gone", [][][2]float64{square(0, 0, 1)}}, // deleted row
	}, map[int]bool{4: true})

	zones, err := Parse(zipped)
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 2 || zones[0].UGC != "GUZ001" || zones[1].UGC != "HIZ001" || zones[1].Name != "Niihau" || zones[0].State != "GU" {
		t.Fatalf("zones %+v", zones)
	}
	var g struct {
		Type        string
		Coordinates [][][2]float64
	}
	if err := json.Unmarshal(zones[0].Rings, &g); err != nil || g.Type != "MultiLineString" || len(g.Coordinates) != 3 {
		t.Fatalf("guam rings %s err %v", zones[0].Rings, err)
	}
	if g.Coordinates[0][1] != [2]float64{145.0, 13.2} {
		t.Errorf("point %v", g.Coordinates[0][1])
	}
}

func TestParseRejectsBadFiles(t *testing.T) {
	good := []rec{{"HI", "001", "Niihau", [][][2]float64{square(-160.3, 21.7, 0.3)}}}
	for name, zipped := range map[string][]byte{
		"not a zip":      []byte("nope"),
		"bad zone code":  shapefile(t, []rec{{"HI", "0A1", "x", good[0].rings}}, nil),
		"point too far":  shapefile(t, []rec{{"HI", "001", "x", [][][2]float64{square(179.9, 0, 1)}}}, nil),
		"ring too short": shapefile(t, []rec{{"HI", "001", "x", [][][2]float64{{{0, 0}, {1, 1}, {0, 0}}}}}, nil),
	} {
		if _, err := Parse(zipped); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

// TestParsePinnedFile reads the real release when it is at hand
// (NWS_ZONES_FILE); CI has no network and skips it.
func TestParsePinnedFile(t *testing.T) {
	path := os.Getenv("NWS_ZONES_FILE")
	if path == "" {
		t.Skip("NWS_ZONES_FILE not set")
	}
	b, err := Zones.Read(t.Context(), nil, path)
	if err != nil {
		t.Fatal(err)
	}
	zones, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, z := range zones {
		have[z.UGC] = true
	}
	for _, ugc := range []string{"HIZ001", "GUZ001", "ASZ001", "MPZ001", "PRZ001", "CAZ006"} {
		if !have[ugc] {
			t.Errorf("no %s", ugc)
		}
	}
	alaska := 0
	for ugc := range have {
		if strings.HasPrefix(ugc, "AKZ") {
			alaska++
		}
	}
	if alaska == 0 {
		t.Error("no Alaska zones")
	}
	t.Logf("%d zones, %d in Alaska", len(zones), alaska)
}
