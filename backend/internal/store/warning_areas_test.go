package store_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"shield/internal/domain"
	"shield/internal/geo/warningareas"
	"shield/internal/store"
	"shield/internal/testutil"
)

func loadWarningAreas(t *testing.T, repo *store.WarningAreaRepository) {
	t.Helper()
	geo, err := os.ReadFile("../geo/warningareas/testdata/geocodes.json")
	if err != nil {
		t.Fatal(err)
	}
	csv, err := os.ReadFile("../geo/warningareas/testdata/aliases.csv")
	if err != nil {
		t.Fatal(err)
	}
	areas, err := warningareas.ParseGeocodes(geo)
	if err != nil {
		t.Fatal(err)
	}
	aliases, err := warningareas.ParseAliases(csv, areas)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Load(context.Background(), areas, aliases, warningareas.SimplifyDegrees, map[string]string{"meteoalarm_areas": "v1"}); err != nil {
		t.Fatal(err)
	}
}

func TestWarningAreaLoadAndResolve(t *testing.T) {
	repo := store.NewWarningAreaRepository(testutil.DB(t))
	ctx := context.Background()
	if v, err := repo.DatasetVersion(ctx, "meteoalarm_areas"); err != nil || v != "" {
		t.Fatalf("version before load %q %v", v, err)
	}
	loadWarningAreas(t, repo)
	// Reloading replaces rather than duplicates.
	loadWarningAreas(t, repo)
	if n, err := repo.Count(ctx); err != nil || n != 3 {
		t.Fatalf("%d areas %v", n, err)
	}
	if v, _ := repo.DatasetVersion(ctx, "meteoalarm_areas"); v != "v1" {
		t.Fatalf("version %q", v)
	}

	emma := domain.AreaCode{Type: domain.AreaTypeEMMA, Value: "BE001"}
	nuts := domain.AreaCode{Type: "NUTS3", Value: "FR712"}
	unknown := domain.AreaCode{Type: "NUTS3", Value: "XX999"}
	notEMMA := domain.AreaCode{Type: "NUTS2", Value: "BE001"} // an EMMA code under another type
	got, err := repo.Codes(ctx, []domain.AreaCode{emma, nuts, unknown, notEMMA})
	if err != nil || len(got) != 2 || got[emma][0] != "BE001" || got[nuts][0] != "FR026" {
		t.Fatalf("codes %v err %v", got, err)
	}

	// A fixed union order: the same parts give byte-identical shapes, so an
	// unchanged warning never looks changed to the event upsert.
	inline := json.RawMessage(`{"type":"Polygon","coordinates":[[[5,50],[5.1,50],[5.1,50.1],[5,50]]]}`)
	a, err := repo.Union(ctx, []string{"BE001", "FR026"}, []json.RawMessage{inline})
	if err != nil || !strings.Contains(string(a), `"MultiPolygon"`) {
		t.Fatalf("union %.80s err %v", a, err)
	}
	b, _ := repo.Union(ctx, []string{"FR026", "BE001"}, []json.RawMessage{inline})
	if string(a) != string(b) {
		t.Fatal("union depends on input order")
	}
	if only, err := repo.Union(ctx, nil, []json.RawMessage{inline}); err != nil || !strings.Contains(string(only), "5.1") {
		t.Fatalf("polygon-only union %s err %v", only, err)
	}
	if _, err := repo.Union(ctx, []string{"XX999"}, nil); err == nil {
		t.Fatal("union of unknown codes succeeded")
	}
	if _, err := repo.Union(ctx, nil, nil); err == nil {
		t.Fatal("empty union succeeded")
	}
}
