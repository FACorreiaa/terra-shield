# Aegis Terra brand assets

These files were imported on 2026-11-16 from the owner's asset pack
(`~/Downloads/Aegis_Terra_Complete_Asset_Library`, manifest v1.1). The sha256 of every copied
file matches `source/MANIFEST.json`. Usage rules are in `DESIGN.md` §11–12. This file only lists
what is here.

| Path | What | Use |
|---|---|---|
| `logo/Aegis_Terra_Original_Logo_Transparent.png` | Full lockup: chrome shield, globe, orbit, "AEGIS TERRA" wordmark, 1254² transparent | Onboarding, About, landing hero, marketing. The app icon and nav mark are a **crop of the shield without the wordmark** on opaque `#080E1A` |
| `mascot/…_Version_B_User_Reference.png` | Sentinel, canonical pose | Onboarding, empty states, About, marketing |
| `mascot/…_Version_A.png` | Sentinel, alternate render | Marketing only |
| `backgrounds/01_Orbital_Earth.{svg,png}` | Wireframe globe + orbit, starfield | Onboarding / About / landing hero |
| `backgrounds/02_Radar_Grid.{svg,png}` | Radar sweep grid | Loading, empty states |
| `backgrounds/03_Precipitation_Atmosphere.{svg,png}` | Rain streaks, isobars, magenta core | Storm / cyclone / rain / flood / wind detail header |
| `backgrounds/04_Wildfire_Contours.{svg,png}` | Contours with warm core | Wildfire / heat / drought / volcano detail header |
| `source/` | The pack's original UI brief, tokens, inspiration links and manifest | Reference only. `design/tokens.json` supersedes the tokens |

Use the SVG backgrounds in the apps (they are small and scale cleanly). The PNGs are for previews.

## Not in the repo

- **3D Sentinel GLB pack** (`06_3D_Production/`, ~12 MB) stays in the asset pack. It is a static
  **reference blockout** (no rig, no UVs, no animation), not a shippable model. A production
  model, plus a USDZ for Apple, is a separate future job with its own brief (the pack's PDF + `docs/`).
- **Third-party inspiration images** were never downloaded. Only links are kept
  (`source/Inspiration_Links_and_Credits.md`).

## Licence and clearance

The logo, mascot and backgrounds are AI-generated originals made for this project. Before the
App Store submission (BACKLOG M4):
- clear the "Aegis Terra" name and mark in the target stores and markets.
- confirm the generation tool's terms allow commercial use of the art.

The app icon crop needs the owner's sign-off.
