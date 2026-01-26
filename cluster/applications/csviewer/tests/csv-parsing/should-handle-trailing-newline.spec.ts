import { test, expect } from '@playwright/test';

test.describe('CSV Parsing - RFC 4180 Compliance', () => {
  test('should handle trailing newline at end of file', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Count all rows
    const tableRows = page.locator('tbody tr');
    const rowCount = await tableRows.count();

    // Trailing newlines should not create empty rows
    // Verify last row has actual content
    const lastRow = tableRows.last();
    const lastRowCells = lastRow.locator('td');
    await expect(lastRowCells).toHaveCount(2);

    // Last row should have non-empty content in first cell
    const lastRowFirstCell = lastRowCells.first();
    const cellText = await lastRowFirstCell.textContent();
    expect(cellText?.trim().length).toBeGreaterThan(0);
  });
});
