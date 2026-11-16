# Aegis Terra mockups

**Live canvas (private, owner's claude.ai):** https://claude.ai/artifact/4KZZm5bbMPCLJLXDaundUV

This is a clickable design canvas made on 2026-11-16 from `DESIGN.md`, `VIEWS.md` and
`design/tokens.json`. It exists for owner verdicts before S3.2. It is not production code.
**Every number, place and event on it is sample data.**

| Artboard | Shows | Interactive |
|---|---|---|
| `Onboarding.dc.html` | 3 steps: logo lockup on Orbital Earth, Sentinel + the four kinds, disclaimer | Continue / Skip |
| `Main.dc.html` | iOS Monitor: map, clusters, markers by severity × kind, extreme pulse, forecast cone, capsule, chips, toast, sheet with stat strip + list | Sheet detent toggle, dismiss toast, rows → detail |
| `EventDetail.dc.html` | iOS event detail: app + source severity, kind, times, key figures, map snippet, source + Open official source, timeline, disclaimer | Scrolls; close → map |
| `Country.dc.html` | iOS Portugal overview: stat strip, type × severity matrix, coverage notice, last 24 h, events | Matrix cells filter the list |
| `Saved.dc.html` | Saved regions (Phase 3) | Tweak `empty` shows the Sentinel empty state |
| `Alerts.dc.html` | Alert center (Phase 3): severity / urgency / certainty as separate fields | Tap marks read; Mark all read |
| `Mac.dc.html` | macOS: sidebar event list, map canvas, floating stat strip, legend, heat-detection layer, inspector | Select events, toggle layer, toggle inspector, time window |

## Files

- `canvas/`: the artboard sources as published. Image `src` values are `/_blob/<id>` asset URLs
  that resolve only inside the artifact. The assets themselves are `design/brand/*`, the map SVGs
  and 18 hazard glyphs.
- `tools/topo2svg.mjs`: builds the dark Natural Earth basemaps (`world-dark.svg`,
  `iberia-dark.svg`) from `world-atlas@2.0.2` TopoJSON, which is public domain:
  `node tools/topo2svg.mjs <dir with land-110m/countries-110m/countries-50m json> <out dir>`.

## Known mock-only liberties

- The basemap is a navy Natural Earth render. That is the look a custom basemap would give.
  MapKit in the MVP will look greyer (DESIGN §9, VIEWS → Open).
- The hazard glyphs are hand-drawn stand-ins for SF Symbols and Lucide.
- Fonts are `system-ui`, which is SF Pro only on Apple devices.
