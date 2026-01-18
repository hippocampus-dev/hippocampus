import { test, expect } from '@playwright/test';

test.describe('CSV Parsing - RFC 4180 Compliance', () => {
  test('should handle commas within quoted fields', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The Lorem ipsum content contains commas within quoted fields
    // Search for Lorem to find rows with comma-containing content
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Multiple rows should be found with Lorem ipsum content
    const tableRows = page.locator('tbody tr');
    const rowCount = await tableRows.count();
    expect(rowCount).toBeGreaterThan(0);

    // Verify the content with commas is displayed correctly in a single cell
    // The Lorem ipsum text contains commas but should be in one cell
    const firstRowFirstCell = tableRows.first().locator('td').first();
    const cellText = await firstRowFirstCell.textContent();
    expect(cellText).toContain(',');
    expect(cellText).toContain('Lorem');
  });
});
