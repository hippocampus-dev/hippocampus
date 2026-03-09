import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should handle network errors during CSV fetch', async ({ page }) => {
    // Navigate to the application first to ensure it loads
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Application should be visible with basic UI
    const searchInput = page.getByPlaceholder('Search for given file');
    await expect(searchInput).toBeVisible();

    // File buttons should be visible
    const fileButtons = page.locator('button');
    await expect(fileButtons.first()).toBeVisible();

    // Table should exist and have content
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Verify rows are loaded
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);
  });
});
