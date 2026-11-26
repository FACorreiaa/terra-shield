import { fileURLToPath } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite-plus";

const apiTarget = process.env["SHIELD_API_URL"] ?? "http://localhost:8080";

export default defineConfig({
  // The router plugin must run before React's so generated routes are transformed too.
  plugins: [tanstackRouter({ target: "react", autoCodeSplitting: true }), react(), tailwindcss()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("src", import.meta.url)) },
  },
  // Same-origin `/v1` in dev, like production behind Caddy: no CORS, no API URL in the bundle.
  // MapLibre 6 runs its worker as an ES module (src/features/map/maplibreWorker.ts).
  worker: { format: "es" },
  // MapLibre alone is ~1.2 MB raw (~350 KB gzip) and loads only on the map routes; the landing
  // page stays far under its budget. Anything bigger than MapLibre still warns.
  build: { chunkSizeWarningLimit: 1300 },
  server: { proxy: { "/v1": apiTarget } },
  preview: { proxy: { "/v1": apiTarget } },
  test: {
    name: "web",
    environment: "happy-dom",
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
