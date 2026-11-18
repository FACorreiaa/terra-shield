#!/bin/sh
# Regenerate shield-api's client from the backend's spec. CI fails if the committed output is stale.
# Pattern: apps/aaru/scripts/generate-api-client.sh.
set -eu
root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/shield-api/Sources/ShieldAPI/Generated"
rm -rf "$out" && mkdir -p "$out"
swift run --package-path "$root/shield-api/Tools" swift-openapi-generator generate \
  "$root/backend/api/openapi.yaml" \
  --config "$root/shield-api/openapi-generator-config.yaml" \
  --output-directory "$out"
