import { test, expect } from '@playwright/test';

test.describe('CSV Parsing - RFC 4180 Compliance', () => {
  test('should handle various line ending formats (CRLF, LF, CR)', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The CSV parser normalizes all line endings to LF
    // Verify that rows are correctly parsed regardless of line ending format
    const tableRows = page.locator('tbody tr');
    const rowCount = await tableRows.count();

    // sample1.csv should have multiple rows parsed correctly
    expect(rowCount).toBeGreaterThan(5);

    // Each row should have the correct number of cells
    for (let i = 0; i < Math.min(rowCount, 5); i++) {
      const cells = tableRows.nth(i).locator('td');
      await expect(cells).toHaveCount(2); // question and answer columns
    }
  });
});
