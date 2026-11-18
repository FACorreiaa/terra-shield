package api

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSpecRefsResolve catches dangling $refs, which the Swift and TS
// generators otherwise report far from the cause.
func TestSpecRefsResolve(t *testing.T) {
	var doc map[string]any
	if err := yaml.Unmarshal(Spec, &doc); err != nil {
		t.Fatal(err)
	}
	var walk func(node any, at string)
	walk = func(node any, at string) {
		switch n := node.(type) {
		case map[string]any:
			for k, v := range n {
				if k == "$ref" {
					ref, _ := v.(string)
					if !resolves(doc, ref) {
						t.Errorf("%s: $ref %q does not resolve", at, ref)
					}
					continue
				}
				walk(v, at+"/"+k)
			}
		case []any:
			for _, v := range n {
				walk(v, at)
			}
		}
	}
	walk(doc, "")
}

func resolves(doc map[string]any, ref string) bool {
	path, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return false
	}
	var node any = doc
	for _, part := range strings.Split(path, "/") {
		m, ok := node.(map[string]any)
		if !ok {
			return false
		}
		if node, ok = m[part]; !ok {
			return false
		}
	}
	return true
}

// TestEventSchemaMatchesClient pins the fields the Swift client decodes
// (shield-core HazardEvent); renaming one breaks the app.
func TestEventSchemaMatchesClient(t *testing.T) {
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(Spec, &doc); err != nil {
		t.Fatal(err)
	}
	event := doc.Components.Schemas["Event"].Properties
	for _, f := range []string{
		"id", "source", "type", "kind", "status", "severity", "sourceSeverity", "severityProvisional",
		"title", "summary", "centroid", "countries", "startedAt", "observedAt", "updatedAt", "endedAt",
		"fetchedAt", "url", "attrs",
	} {
		if _, ok := event[f]; !ok {
			t.Errorf("Event.%s missing", f)
		}
	}
}
