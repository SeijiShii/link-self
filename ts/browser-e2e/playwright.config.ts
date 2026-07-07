import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "tests",
  timeout: 60_000,
  // Keep resource usage low: a single Chromium worker.
  workers: 1,
  use: {
    baseURL: "http://localhost:5199",
    headless: true,
  },
  webServer: {
    command: "npx vite",
    url: "http://localhost:5199",
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
