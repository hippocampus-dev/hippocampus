import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should wrap long text and preserve line breaks', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for rows with long content
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Get a cell with long content
    const firstCell = page.locator('tbody tr').first().locator('td').first();
    await expect(firstCell).toBeVisible();

    // Cell should have whitespace-pre-line for preserving line breaks
    await expect(firstCell).toHaveClass(/whitespace-pre-line/);

    // Cell should have break-all for word breaking
    await expect(firstCell).toHaveClass(/break-all/);

    // Verify content is visible (not cut off)
    const cellText = await firstCell.textContent();
    expect(cellText?.length).toBeGreaterThan(100);
  });
});
