import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Basic Search', () => {
  test('should show all rows when search query is empty', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Get initial row count with empty search
    const initialRowCount = await page.locator('tbody tr').count();
    expect(initialRowCount).toBeGreaterThan(0);

    const searchInput = page.getByPlaceholder('Search for given file');

    // Apply a filter
    await searchInput.fill('aaa');
    await page.waitForTimeout(300);
    const filteredCount = await page.locator('tbody tr').count();
    expect(filteredCount).toBeLessThan(initialRowCount);

    // Clear the search
    await searchInput.fill('');
    await page.waitForTimeout(300);

    // All rows should be displayed again
    const clearedCount = await page.locator('tbody tr').count();
    expect(clearedCount).toBe(initialRowCount);
  });
});
