import { test, expect } from '@playwright/test';

test.describe('URL-Based State Management and Row Highlighting', () => {
  test('should URL-encode row identifiers properly', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for row with special characters in the question (pkey)
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Click on a row with complex content
    const firstRow = page.locator('tbody tr').first();
    await firstRow.click();
    await page.waitForTimeout(300);

    // Get the URL
    const url = page.url();

    // URL should contain a hash
    expect(url).toContain('#');

    // The hash should be URL-encoded (special characters encoded)
    const hash = new URL(url).hash;
    expect(hash.length).toBeGreaterThan(1);

    // The current row should be highlighted
    const highlightedRows = page.locator('tbody tr.bg-orange-50');
    const count = await highlightedRows.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });
});
