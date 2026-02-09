import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should display table rows with stored column data', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Rows should be present
    const rows = page.locator('tbody tr');
    const rowCount = await rows.count();
    expect(rowCount).toBeGreaterThan(0);

    // Each row should have correct number of cells
    const firstRowCells = rows.first().locator('td');
    await expect(firstRowCells).toHaveCount(2);

    // Cells should have content
    const firstCell = firstRowCells.first();
    const cellText = await firstCell.textContent();
    expect(cellText?.length).toBeGreaterThan(0);

    // Cells should have proper styling
    await expect(firstCell).toHaveClass(/text-lg/);
    await expect(firstCell).toHaveClass(/text-orange-600/);
  });
});
