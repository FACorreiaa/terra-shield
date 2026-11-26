import { defineConfig, devices } from "@playwright/test";

/** `E2E_PORT` moves it when another dev server already holds 4173 (it would be reused and tested instead). */
const port = Number(process.env["E2E_PORT"] ?? "4173");
const isCI = process.env["CI"] !== undefined;

/** Smoke tests against the production build (`vp preview`), API mocked per test. */
export default defineConfig({
  testDir: "e2e",
  forbidOnly: isCI,
  retries: isCI ? 1 : 0,
  reporter: isCI ? [["github"], ["list"]] : "list",
  use: { baseURL: `http://localhost:${port}`, trace: "on-first-retry" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: `vp preview --port ${port} --strictPort`,
    port,
    reuseExistingServer: !isCI,
  },
});
