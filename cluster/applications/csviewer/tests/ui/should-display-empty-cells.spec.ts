import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should display empty cells correctly', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for rows that might have empty cells
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Get all rows
    const rows = page.locator('tbody tr');
    const rowCount = await rows.count();
    expect(rowCount).toBeGreaterThan(0);

    // Verify table structure is maintained even with potentially empty cells
    for (let i = 0; i < Math.min(rowCount, 3); i++) {
      const cells = rows.nth(i).locator('td');
      await expect(cells).toHaveCount(2);

      // Each cell should exist (even if empty)
      for (let j = 0; j < 2; j++) {
        const cell = cells.nth(j);
        await expect(cell).toBeVisible();
      }
    }
  });
});
