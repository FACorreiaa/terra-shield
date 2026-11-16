// Package features lists every API feature, in mount order.
package features

import (
	"shield/internal/features/ops"
	"shield/internal/features/sources"
	"shield/internal/server"
)

// All returns every feature.
func All() []server.Feature {
	return []server.Feature{
		ops.Feature{},
		sources.Feature{},
	}
}
