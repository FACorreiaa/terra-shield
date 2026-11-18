package app

import (
	"regexp"
	"strings"
	"testing"

	"shield/api"
	"shield/internal/config"

	"gopkg.in/yaml.v3"
)

var echoParam = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// TestRoutesInSpec keeps openapi.yaml honest: every route the server
// registers must be documented, because the Swift and TS clients are
// generated from the spec.
func TestRoutesInSpec(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(api.Spec, &spec); err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	for path, ops := range spec.Paths {
		for method := range ops {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}

	routes, err := Routes(config.Base{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) == 0 {
		t.Fatal("no routes registered")
	}
	for _, r := range routes {
		method, path, _ := strings.Cut(r, " ")
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			continue // echo's internal any/not-found entries
		}
		key := method + " " + echoParam.ReplaceAllString(path, "{$1}")
		if !documented[key] {
			t.Errorf("route %s is not in api/openapi.yaml", key)
		}
	}
}
