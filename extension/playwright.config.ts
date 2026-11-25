import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : "50%",
  reporter: "html",

  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        channel: "chrome",
        launchOptions: {
          // Without --single-process, which crashes Chrome when JavaScript dialogs open in parallel workers
          args: ["--no-sandbox", "--disable-setuid-sandbox", "--no-zygote"],
        },
      },
    },
  ],
});
