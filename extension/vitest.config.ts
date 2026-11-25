import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // The default include also matches the Playwright specs under tests/: https://vitest.dev/config/include
    include: ["test/**/*.test.ts"],
  },
});
