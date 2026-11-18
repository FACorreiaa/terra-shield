package countries_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
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

func TestFetchVerifiesChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	}))
	defer srv.Close()

	_, err := countries.Fetch(t.Context(), srv.Client(), srv.URL, countries.DatasetSHA256)
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err %v, want checksum mismatch", err)
	}
	// The matching hash passes.
	sum := sha256.Sum256([]byte("tampered"))
	body, err := countries.Fetch(t.Context(), srv.Client(), srv.URL, hex.EncodeToString(sum[:]))
	if err != nil || string(body) != "tampered" {
		t.Fatalf("body %q err %v", body, err)
	}
}
