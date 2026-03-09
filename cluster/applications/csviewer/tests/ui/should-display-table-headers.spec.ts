import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should display table headers from stored column names', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Headers should be present
    const headerRow = page.locator('thead tr');
    await expect(headerRow).toBeVisible();

    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2);

    // Verify header text matches stored column configuration
    await expect(headerCells.nth(0)).toHaveText('question');
    await expect(headerCells.nth(1)).toHaveText('answer');

    // Headers should have correct styling (white text on orange background)
    const thead = page.locator('thead');
    await expect(thead).toHaveClass(/bg-orange-700/);
    await expect(thead).toHaveClass(/text-white/);
  });
});
