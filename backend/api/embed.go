// Package api embeds the OpenAPI document, the contract for the Swift and
// TypeScript clients.
package api

import _ "embed"

// Spec is openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
