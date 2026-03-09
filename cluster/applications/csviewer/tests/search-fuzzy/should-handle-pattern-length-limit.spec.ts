import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Fuzzy Search (Bitap Algorithm)', () => {
  test('should handle pattern length limit (64 characters)', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // Create a pattern longer than 64 characters
    const longPattern = 'a'.repeat(70);
    await searchInput.fill(longPattern);
    await page.waitForTimeout(300);

    // Application should not crash - table should still be visible
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Search input should have the full value
    await expect(searchInput).toHaveValue(longPattern);

    // For patterns > 64 chars, bitap falls back to includes() exact match
    // This should still work without errors
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    // Long pattern won't match anything, so count should be 0
    expect(count).toBe(0);

    // Verify we can still search normally after long pattern
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);
    const normalCount = await page.locator('tbody tr').count();
    expect(normalCount).toBeGreaterThan(0);
  });
});
