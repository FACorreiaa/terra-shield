package countries_test

import (
	"os"
	"strings"
	"testing"

	"shield/internal/domain"
	"shield/internal/geo/countries"
)

func parseFixture(t *testing.T) ([]domain.Country, []string) {
	t.Helper()
	f, err := os.Open("testdata/countries.geojson")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	features, skipped, err := countries.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return features, skipped
}

func TestParseAppliesOverrides(t *testing.T) {
	features, skipped := parseFixture(t)
	if len(skipped) != 1 || skipped[0] != "Siachen Glacier" {
		t.Fatalf("skipped %v", skipped)
	}
	codes := map[string]string{}
	for _, f := range features {
		codes[f.Name] = f.ISO2 + "/" + f.ISO3
	}
	for name, want := range map[string]string{
		"Somaliland": "SO/SOM",
		"Kosovo":     "XK/XKX",
		"Portugal":   "PT/PRT",
	} {
		if codes[name] != want {
			t.Errorf("%s = %q, want %q", name, codes[name], want)
		}
	}
}

func TestParseRejectsUnknownCodes(t *testing.T) {
	in := `{"features":[{"properties":{"ADMIN":"Nowhere","ISO_A2_EH":"-99","ISO_A3_EH":"-99"},
		"geometry":{"type":"Point","coordinates":[0,0]}}]}`
	if _, _, err := countries.Parse(strings.NewReader(in)); err == nil {
		t.Fatal("want error for a feature with no ISO codes and no override")
	}
}
