import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Basic Search', () => {
  test('should display no results when no matches found', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // Enter a search term that matches no rows
    await searchInput.fill('xyznonexistent12345');
    await page.waitForTimeout(300);

    // Table body should be empty
    const tableRows = page.locator('tbody tr');
    await expect(tableRows).toHaveCount(0);

    // Application should not crash - search input should still be functional
    await expect(searchInput).toBeVisible();
    await expect(searchInput).toBeEnabled();

    // Table structure should still exist
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Headers should still be visible
    const headerCells = page.locator('thead th');
    await expect(headerCells.first()).toBeVisible();
  });
});
