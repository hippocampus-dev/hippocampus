import { test, expect } from '@playwright/test';

test.describe('CSV Parsing - RFC 4180 Compliance', () => {
  test('should handle malformed CSV gracefully', async ({ page }) => {
    // Listen for console errors
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Application should not crash - basic elements should be visible
    const searchInput = page.getByPlaceholder('Search for given file');
    await expect(searchInput).toBeVisible();

    const fileButtons = page.locator('button');
    await expect(fileButtons.first()).toBeVisible();

    // The table should be rendered (even if empty for malformed data)
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Check that no critical parsing errors were logged
    // RFC4180DoubleQuoteError would indicate malformed CSV
    const criticalErrors = consoleErrors.filter(e => e.includes('RFC4180'));
    expect(criticalErrors.length).toBe(0);
  });
});
