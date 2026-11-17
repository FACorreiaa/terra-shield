import { defineConfig } from "vite-plus";

export default defineConfig({
  test: { name: "api-client", include: ["src/**/*.test.ts"] },
});
