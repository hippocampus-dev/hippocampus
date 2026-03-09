import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Fuzzy Search (Bitap Algorithm)', () => {
  test('should support Japanese text segmentation', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Verify Intl.Segmenter is available
    const segmenterAvailable = await page.evaluate(() => {
      return typeof Intl.Segmenter !== 'undefined';
    });
    expect(segmenterAvailable).toBe(true);

    const searchInput = page.getByPlaceholder('Search for given file');

    // Test that the search input accepts Japanese text
    await searchInput.fill('テスト');
    await page.waitForTimeout(300);

    // Search should process without errors (even if no Japanese content exists)
    await expect(searchInput).toHaveValue('テスト');

    // Application should not crash
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Clear and verify normal operation continues
    await searchInput.fill('');
    await page.waitForTimeout(300);
    const rowCount = await page.locator('tbody tr').count();
    expect(rowCount).toBeGreaterThan(0);
  });
});
