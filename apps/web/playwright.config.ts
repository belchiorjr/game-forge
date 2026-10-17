import { defineConfig } from "@playwright/test";

// Run through `make e2e`, which starts an isolated API + web and sets E2E_BASE_URL.
export default defineConfig({
  testDir: "e2e",
  timeout: 90_000,
  expect: { timeout: 30_000 },
  workers: 1, // tests share one API instance and run against a real agent loop
  reporter: [["list"]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3300",
    trace: "retain-on-failure",
    launchOptions: { args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"] }, // WebGL for the WASM game
  },
});
