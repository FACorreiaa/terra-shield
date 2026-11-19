package warningareas_test

import (
	"os"
	"strings"
	"testing"

	"shield/internal/domain"
	"shield/internal/geo/warningareas"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The fixture is three areas cut from the pinned 2026-11-13 release.
func TestParseRecordedFiles(t *testing.T) {
	areas, err := warningareas.ParseGeocodes(read(t, "geocodes.json"))
	if err != nil || len(areas) != 3 {
		t.Fatalf("%d areas, err %v", len(areas), err)
	}
	if a := areas[0]; a.Code != "BE001" || a.Country != "BE" || a.Name != "Luxembourg" || !strings.Contains(string(a.Geometry), "Polygon") {
		t.Fatalf("first area %+v", a)
	}
	aliases, err := warningareas.ParseAliases(read(t, "aliases.csv"), areas)
	want := []domain.AreaAlias{{Type: "NUTS2", Alias: "BE34", Code: "BE001"}, {Type: "NUTS3", Alias: "FR712", Code: "FR026"}}
	if err != nil || len(aliases) != 2 || aliases[0] != want[0] || aliases[1] != want[1] {
		t.Fatalf("aliases %+v err %v", aliases, err)
	}
}

func TestParseGeocodesRejectsBadFiles(t *testing.T) {
	poly := `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}`
	feature := func(code, country, typ, geom string) string {
		return `{"type":"Feature","properties":{"code":"` + code + `","country":"` + country + `","name":"N","type":"` + typ + `"},"geometry":` + geom + `}`
	}
	fc := func(fs ...string) []byte {
		return []byte(`{"type":"FeatureCollection","features":[` + strings.Join(fs, ",") + `]}`)
	}
	cases := map[string][]byte{
		"empty":        fc(),
		"not json":     []byte("{"),
		"bad code":     fc(feature("be001", "BE", "EMMA_ID", poly)),
		"bad country":  fc(feature("BE001", "Belgium", "EMMA_ID", poly)),
		"not EMMA_ID":  fc(feature("BE001", "BE", "NUTS2", poly)),
		"duplicate":    fc(feature("BE001", "BE", "EMMA_ID", poly), feature("BE001", "BE", "EMMA_ID", poly)),
		"point":        fc(feature("BE001", "BE", "EMMA_ID", `{"type":"Point","coordinates":[0,0]}`)),
		"no geometry":  fc(feature("BE001", "BE", "EMMA_ID", `null`)),
		"wrong object": []byte(`{"type":"Feature"}`),
	}
	for name, body := range cases {
		if _, err := warningareas.ParseGeocodes(body); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

func TestParseAliasesRejectsBadRows(t *testing.T) {
	known := []domain.WarningArea{{Code: "BE001"}}
	cases := map[string]string{
		"bad header":   "A,B,C\nBE001,BE34,NUTS2\n",
		"unknown area": "CODE,ALIAS_CODE,ALIAS_TYPE\nBE999,BE34,NUTS2\n",
		"short row":    "CODE,ALIAS_CODE,ALIAS_TYPE\nBE001,BE34\n",
		"EMMA alias":   "CODE,ALIAS_CODE,ALIAS_TYPE\nBE001,BE002,EMMA_ID\n",
		"bad alias":    "CODE,ALIAS_CODE,ALIAS_TYPE\nBE001,BE 34,NUTS2\n",
	}
	for name, body := range cases {
		if _, err := warningareas.ParseAliases([]byte(body), known); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}
