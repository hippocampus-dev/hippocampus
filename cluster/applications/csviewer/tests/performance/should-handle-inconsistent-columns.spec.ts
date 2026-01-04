import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should handle CSV with inconsistent column counts', async ({ page }) => {
    // Listen for errors
    const pageErrors: string[] = [];
    page.on('pageerror', err => {
      pageErrors.push(err.message);
    });

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Application should handle any inconsistent data gracefully
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Headers should be consistent
    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2);

    // Rows should be rendered (even with potential missing columns)
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);

    // No page errors should occur
    expect(pageErrors.length).toBe(0);
  });
});
