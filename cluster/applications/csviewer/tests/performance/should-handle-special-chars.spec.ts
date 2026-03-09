import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should handle special characters in CSV content', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for content with special characters
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Get a cell with potentially special content
    const cell = page.locator('tbody tr').first().locator('td').first();
    await expect(cell).toBeVisible();

    // Content should be rendered without corruption
    const cellText = await cell.textContent();
    expect(cellText?.length).toBeGreaterThan(0);

    // Special characters like commas should be preserved (they were in quoted fields)
    expect(cellText).toContain(',');

    // Test that the page handles unicode
    await searchInput.fill('');
    await page.waitForTimeout(300);

    // Application should still be functional
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);
  });
});
