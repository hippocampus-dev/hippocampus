import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should handle empty CSV file', async ({ page }) => {
    // Listen for page errors
    const pageErrors: string[] = [];
    page.on('pageerror', err => {
      pageErrors.push(err.message);
    });

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Filter to show no results (simulates empty data)
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('xyznonexistent12345');
    await page.waitForTimeout(300);

    // Table structure should still exist
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Headers should still be visible
    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2);

    // Body should be empty but not cause errors
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(0);

    // No JavaScript errors should occur
    expect(pageErrors.length).toBe(0);

    // Application should remain functional
    await searchInput.fill('');
    await page.waitForTimeout(300);

    // Rows should reappear
    const allRows = await page.locator('tbody tr').count();
    expect(allRows).toBeGreaterThan(0);
  });
});
