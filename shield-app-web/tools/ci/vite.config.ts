import { defineConfig } from "vite-plus";

export default defineConfig({
  test: { name: "ci", include: ["src/**/*.test.ts"] },
});
