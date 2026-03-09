import { test, expect } from '@playwright/test';

test.describe('Browser Compatibility and CDN Dependencies', () => {
  test('should load TailwindCSS from CDN successfully', async ({ page }) => {
    // Track TailwindCSS requests
    const tailwindRequests: string[] = [];
    page.on('request', request => {
      if (request.url().includes('cdn.tailwindcss.com')) {
        tailwindRequests.push(request.url());
      }
    });

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Verify TailwindCSS was requested
    expect(tailwindRequests.length).toBeGreaterThan(0);

    // Verify Tailwind classes are applied - check computed styles
    const searchInput = page.getByPlaceholder('Search for given file');
    await expect(searchInput).toBeVisible();

    // Check that Tailwind classes are working (element should have rounded corners)
    const borderRadius = await searchInput.evaluate(el => {
      return window.getComputedStyle(el).borderRadius;
    });
    // rounded-md in Tailwind equals 0.375rem or 6px
    expect(borderRadius).not.toBe('0px');

    // Check background color on header (bg-orange-700)
    const thead = page.locator('thead');
    const bgColor = await thead.evaluate(el => {
      return window.getComputedStyle(el).backgroundColor;
    });
    // bg-orange-700 should produce an orange color
    expect(bgColor).not.toBe('rgba(0, 0, 0, 0)');
    expect(bgColor).toContain('rgb');
  });
});
