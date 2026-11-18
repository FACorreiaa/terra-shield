package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

func TestCached(t *testing.T) {
	e := echo.New()
	body := []byte(`{"a":1}`)
	tag := ETag(body)
	run := func(inm string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		if inm != "" {
			req.Header.Set("If-None-Match", inm)
		}
		rec := httptest.NewRecorder()
		if err := Cached(e.NewContext(req, rec), "application/json", body, tag, 15*time.Second); err != nil {
			t.Fatal(err)
		}
		return rec
	}

	rec := run("")
	if rec.Code != http.StatusOK || rec.Body.String() != `{"a":1}` || rec.Header().Get("ETag") != tag ||
		rec.Header().Get("Cache-Control") != "public, max-age=15" || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("200: %d %v %q", rec.Code, rec.Header(), rec.Body)
	}
	for _, inm := range []string{tag, "W/" + tag, `"other", ` + tag, "*"} {
		if rec := run(inm); rec.Code != http.StatusNotModified || rec.Body.Len() != 0 || rec.Header().Get("ETag") != tag {
			t.Errorf("If-None-Match %s: %d", inm, rec.Code)
		}
	}
	if rec := run(`"other"`); rec.Code != http.StatusOK {
		t.Errorf("mismatch: %d", rec.Code)
	}
}
