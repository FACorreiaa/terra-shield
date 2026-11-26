package usgsvolcano

import (
	"context"
	"strings"
	"testing"

	"shield/internal/domain"
	"shield/internal/store"
	"shield/internal/testutil"
)

// Against PostGIS: Kilauea elevated, then back at normal (ended), then
// elevated again (active once more).
func TestPollAgainstStore(t *testing.T) {
	pool := testutil.DB(t)
	ctx := context.Background()
	if err := store.NewSourceRepository(pool).Register(ctx, New(nil, nil, nil).Info()); err != nil {
		t.Fatal(err)
	}
	events := store.NewEventRepository(pool)
	elevated := read(t, "elevated.json")
	status := string(read(t, "geojson.json"))
	i := strings.Index(status, `"volcanoCd":"hi3"`)
	quiet := status[:i] + strings.Replace(status[i:], `"alertLevel":"ADVISORY","colorCode":"YELLOW"`, `"alertLevel":"NORMAL","colorCode":"GREEN"`, 1)
	if i < 0 || quiet == status {
		t.Fatal("fixture edit missed")
	}
	kilauea := func(body string) string {
		t.Helper()
		if _, err := newSource(t, []byte(body), elevated, events).Poll(ctx, domain.Validators{}); err != nil {
			t.Fatal(err)
		}
		var st string
		if err := pool.QueryRow(ctx, `SELECT status FROM events WHERE source_id = $1 AND source_event_id = 'hi3'`, ID).Scan(&st); err != nil {
			t.Fatal(err)
		}
		return st
	}
	if st := kilauea(status); st != "active" {
		t.Fatalf("first poll: %s", st)
	}
	if st := kilauea(quiet); st != "resolved" {
		t.Fatalf("back at normal: %s", st)
	}
	if st := kilauea(status); st != "active" {
		t.Fatalf("elevated again: %s", st)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE source_id = $1 AND status = 'active'`, ID).Scan(&n); err != nil || n != 3 {
		t.Fatalf("%d active, %v", n, err)
	}
}
