import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Basic Search', () => {
  test('should trim whitespace-only queries', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Get initial row count
    const initialRowCount = await page.locator('tbody tr').count();

    const searchInput = page.getByPlaceholder('Search for given file');

    // Enter whitespace-only query
    await searchInput.fill('   ');
    await page.waitForTimeout(300);

    // Whitespace-only query should be treated as empty
    const whitespaceCount = await page.locator('tbody tr').count();
    expect(whitespaceCount).toBe(initialRowCount);

    // Try with tabs and mixed whitespace
    await searchInput.fill('  \t  ');
    await page.waitForTimeout(300);

    const mixedWhitespaceCount = await page.locator('tbody tr').count();
    expect(mixedWhitespaceCount).toBe(initialRowCount);
  });
});
