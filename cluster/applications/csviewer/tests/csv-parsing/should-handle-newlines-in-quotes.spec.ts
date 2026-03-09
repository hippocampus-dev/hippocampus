import { test, expect } from '@playwright/test';

test.describe('CSV Parsing - RFC 4180 Compliance', () => {
  test('should handle newlines within quoted fields', async ({ page }) => {
    // Navigate to the application (sample1.csv has "fuga\nnextline" in paraphrases)
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for the row containing hogeeeeeeee (which has multi-line content)
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('hogeeeeeeee');
    await page.waitForTimeout(300);

    // The row should be found (newlines in quoted fields should not split the row)
    const tableRows = page.locator('tbody tr');
    await expect(tableRows).toHaveCount(1);

    // Verify the question column shows hogeeeeeeee
    const firstCell = tableRows.first().locator('td').first();
    await expect(firstCell).toHaveText('hogeeeeeeee');
  });
});
