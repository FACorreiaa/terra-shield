package httpx_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"shield/internal/httpx"

	"github.com/labstack/echo/v5"
)

func TestStatusOf(t *testing.T) {
	e := echo.New()
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"no error", nil, http.StatusOK},
		{"abort", httpx.NotFound("gone"), http.StatusNotFound},
		{"echo error", echo.ErrMethodNotAllowed, http.StatusMethodNotAllowed},
		{"plain error", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := e.NewContext(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), httptest.NewRecorder())
			if got := httpx.StatusOf(ctx, c.err); got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}
