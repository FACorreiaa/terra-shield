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
  server: { proxy: { "/v1": apiTarget } },
  preview: { proxy: { "/v1": apiTarget } },
  test: {
    name: "web",
    environment: "happy-dom",
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
