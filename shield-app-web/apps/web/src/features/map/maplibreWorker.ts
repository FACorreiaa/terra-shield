import { setWorkerUrl } from "maplibre-gl";
import workerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url";

// MapLibre's stylesheet travels with the map, so pages without one (the landing page) never load it.
import "maplibre-gl/dist/maplibre-gl.css";

/**
 * MapLibre 6 finds its worker next to its own module (`import.meta.url`), which bundling breaks.
 * Vite emits the worker as its own ES chunk instead; MapLibre is told where it went.
 */
function registerBundledWorker(): void {
  setWorkerUrl(workerUrl);
}

export { registerBundledWorker };
