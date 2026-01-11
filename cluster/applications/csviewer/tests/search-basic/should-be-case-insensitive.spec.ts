import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Basic Search', () => {
  test('should perform case-insensitive search', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // Search with lowercase
    await searchInput.fill('lorem');
    await page.waitForTimeout(300);
    const lowercaseCount = await page.locator('tbody tr').count();

    // Clear and search with uppercase
    await searchInput.fill('LOREM');
    await page.waitForTimeout(300);
    const uppercaseCount = await page.locator('tbody tr').count();

    // Clear and search with mixed case
    await searchInput.fill('LoReM');
    await page.waitForTimeout(300);
    const mixedCaseCount = await page.locator('tbody tr').count();

    // All should return the same number of results
    expect(lowercaseCount).toBe(uppercaseCount);
    expect(lowercaseCount).toBe(mixedCaseCount);
    expect(lowercaseCount).toBeGreaterThan(0);
  });
});
