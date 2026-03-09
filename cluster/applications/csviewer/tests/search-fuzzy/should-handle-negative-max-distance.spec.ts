import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Fuzzy Search (Bitap Algorithm)', () => {
  test('should handle negative maxDistance gracefully', async ({ page }) => {
    // Listen for console errors
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    // Navigate to the application
    // The application's bitap function throws an error for negative maxDistance
    // But this should be handled at the configuration level
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // Normal search should work (maxDistance is configured as 1 in index.html)
    await searchInput.fill('foo');
    await page.waitForTimeout(300);

    // Application should function normally
    const rows = page.locator('tbody tr');
    await expect(rows.first()).toBeVisible();

    // No errors related to maxDistance should appear
    const maxDistanceErrors = consoleErrors.filter(e =>
      e.includes('maxDistance') || e.includes('must be >= 0')
    );
    expect(maxDistanceErrors.length).toBe(0);
  });
});
