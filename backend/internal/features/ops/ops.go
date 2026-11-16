// Package ops serves the OpenAPI document.
package ops

import (
	"net/http"

	"shield/api"
	"shield/internal/server"

	"github.com/labstack/echo/v5"
)

// Feature serves /openapi.yaml.
type Feature struct{}

// Name implements server.Feature.
func (Feature) Name() string { return "ops" }

// Register implements server.Feature.
func (Feature) Register(_ *server.Deps, r *server.Routes) error {
	r.Root.GET("/openapi.yaml", func(c *echo.Context) error {
		return c.Blob(http.StatusOK, "application/yaml", api.Spec)
	})
	return nil
}
