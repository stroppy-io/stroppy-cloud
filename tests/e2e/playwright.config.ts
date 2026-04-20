import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "*.spec.ts",
  timeout: 120_000,
  retries: 0,
  workers: 1, // sequential — tests share state (login, runs)
  use: {
    baseURL: process.env.BASE_URL || "http://localhost:8080",
    headless: true,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    browserName: "firefox",
  },
  reporter: [["list"], ["html", { open: "never", outputFolder: "report" }]],
});
