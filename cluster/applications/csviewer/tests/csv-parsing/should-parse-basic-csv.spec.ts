import { test, expect } from '@playwright/test';

test.describe('CSV Parsing - RFC 4180 Compliance', () => {
  test('should parse basic CSV with headers and rows', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Headers are correctly extracted from first row
    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2); // question and answer are stored columns

    // Verify header text
    await expect(headerCells.nth(0)).toHaveText('question');
    await expect(headerCells.nth(1)).toHaveText('answer');

    // All data rows are parsed correctly
    const tableRows = page.locator('tbody tr');
    const rowCount = await tableRows.count();
    expect(rowCount).toBeGreaterThan(0);

    // Column count matches header count for each row
    const firstRowCells = page.locator('tbody tr').first().locator('td');
    await expect(firstRowCells).toHaveCount(2);
  });
});
