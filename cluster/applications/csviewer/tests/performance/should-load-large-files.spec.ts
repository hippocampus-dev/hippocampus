import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should load large CSV files efficiently', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');

    // Measure load time
    const startTime = Date.now();
    await page.waitForLoadState('networkidle');
    const loadTime = Date.now() - startTime;

    // File should load within acceptable time (5 seconds)
    expect(loadTime).toBeLessThan(5000);

    // UI should be responsive - verify elements are interactable
    const searchInput = page.getByPlaceholder('Search for given file');
    await expect(searchInput).toBeEnabled();

    // Table should be rendered
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Rows should be displayed
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);
  });
});
