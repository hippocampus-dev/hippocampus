import { expect, test } from "@playwright/test";

test.describe("Browser Compatibility and CDN Dependencies", () => {
  test("should load TailwindCSS from CDN successfully", async ({ page }) => {
    // Track TailwindCSS requests
    const tailwindRequests: string[] = [];
    page.on("request", (request) => {
      if (request.url().includes("cdn.tailwindcss.com")) {
        tailwindRequests.push(request.url());
      }
    });

    // Navigate to the application
    await page.goto("/");
    await page.waitForLoadState("networkidle");

    // Verify TailwindCSS was requested
    expect(tailwindRequests.length).toBeGreaterThan(0);

    // Verify Tailwind classes are applied - check computed styles
    const searchInput = page.getByPlaceholder("Search for given file");
    await expect(searchInput).toBeVisible();

    // Check that Tailwind classes are working (element should have rounded corners)
    const borderRadius = await searchInput.evaluate((element) => {
      return window.getComputedStyle(element).borderRadius;
    });
    // rounded-md in Tailwind equals 0.375rem or 6px
    expect(borderRadius).not.toBe("0px");

    // Check background color on header (bg-orange-700)
    const thead = page.locator("thead");
    const backgroundColor = await thead.evaluate((element) => {
      return window.getComputedStyle(element).backgroundColor;
    });
    // bg-orange-700 should produce an orange color
    expect(backgroundColor).not.toBe("rgba(0, 0, 0, 0)");
    expect(backgroundColor).toContain("rgb");
  });
});
