import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Basic Search', () => {
  test('should find exact matches in indexed columns', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Get initial row count
    const initialRowCount = await page.locator('tbody tr').count();
    expect(initialRowCount).toBeGreaterThan(0);

    // Enter an exact match search term
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('foo');
    await page.waitForTimeout(300);

    // Only rows containing the term should be displayed
    const filteredRows = page.locator('tbody tr');
    const filteredCount = await filteredRows.count();
    expect(filteredCount).toBeLessThan(initialRowCount);
    expect(filteredCount).toBeGreaterThan(0);

    // Verify at least the first row contains 'foo' in the question column
    const firstCell = filteredRows.first().locator('td').first();
    const cellText = await firstCell.textContent();
    expect(cellText?.toLowerCase()).toContain('foo');
  });
});
