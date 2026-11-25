import { expect, test } from "@playwright/test";

test.describe("Browser Compatibility and CDN Dependencies", () => {
  test("should function with ES modules browser support", async ({ page }) => {
    // Listen for module-related errors
    const moduleErrors: string[] = [];
    page.on("console", (message) => {
      if (
        message.type() === "error" &&
        message.text().toLowerCase().includes("module")
      ) {
        moduleErrors.push(message.text());
      }
    });

    page.on("pageerror", (error) => {
      if (
        error.message.toLowerCase().includes("module") ||
        error.message.toLowerCase().includes("import")
      ) {
        moduleErrors.push(error.message);
      }
    });

    // Navigate to the application
    await page.goto("/");
    await page.waitForLoadState("networkidle");

    // Verify no module errors occurred
    expect(moduleErrors.length).toBe(0);

    // Verify the script type="module" is being used
    const moduleScript = page.locator('script[type="module"]');
    await expect(moduleScript).toHaveCount(1);

    // Verify the application is functional (modules loaded correctly)
    const table = page.locator("table");
    await expect(table).toBeVisible();

    const rows = page.locator("tbody tr");
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);
  });
});
