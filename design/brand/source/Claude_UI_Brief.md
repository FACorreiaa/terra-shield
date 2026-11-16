# Aegis Terra | Claude UI implementation brief

Create an original premium global natural disaster monitoring app for iOS, macOS and web. Use the bundled transparent Aegis Terra logo and Sentinel mascot, and the original vector backgrounds (`03_Backgrounds`) as optional hero / onboarding imagery. Do not use the mascot on the main map if it would interfere with hazard legibility.

## App structure
- Global live map with severity/category/country filters, source and last-updated indicators.
- Country overview with status, time horizon and verified source coverage.
- Disaster detail panel with location, active vs resolved status, map boundary, source references and update timeline.
- Saved regions and opt-in notification preferences.
- Accessible alert center with urgency, certainty and severity clearly distinguished.

## Apple and web
- SwiftUI native iOS full-screen map, filter chips, incident sheet, dynamic type, system accessibility.
- SwiftUI native macOS navigation sidebar, large map canvas, contextual inspector and keyboard shortcuts.
- SvelteKit + TypeScript responsive web dashboard, MapLibre GL JS map, dark mode with accessible contrast.

## Look and feel
Near-black navy (#080E1A), graphite panels (#111C2D), slate (#203550), informational cyan (#52B6FF), amber warnings (#FFB54A), destructive red (#F35D62). Reserve alert colors for actual severity states. Typography: Apple system font on native; system-ui / Inter-like web fallback (don't bundle proprietary fonts). Moderate-radius cards, exceptionally clear density, spare visual effects.

## Architecture reference
Go (Echo) API, pgx + sqlc, PostgreSQL + PostGIS, River jobs, deploy to Hetzner K3s. Prefer OpenAPI contracts and SQL migrations. Store source severity separately from app severity; distinguish observed events, official warnings, and forecasts.

## Guardrails
Use realistic but clearly marked fake demo values; never present mock data as a live warning. Show source, coverage, and freshness. Do not copy third-party UIs, brand logos, cartographic tiles or imagery without a license.
