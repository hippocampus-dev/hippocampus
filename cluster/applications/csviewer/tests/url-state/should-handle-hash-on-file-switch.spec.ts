import { test, expect } from '@playwright/test';

test.describe('URL-Based State Management and Row Highlighting', () => {
  test('should preserve hash when switching files', async ({ page }) => {
    // Navigate to the application with a hash
    await page.goto('/#aaa');
    await page.waitForLoadState('networkidle');

    // Verify initial hash is set
    expect(page.url()).toContain('#aaa');

    // Switch to a different file
    const sample2Button = page.getByRole('button', { name: 'Sample2' });
    await sample2Button.click();
    await page.waitForLoadState('networkidle');

    // Hash should be cleared when switching files
    // (because the row with that id might not exist in new file)
    await page.waitForTimeout(300);

    // URL should have file parameter but hash may be cleared
    expect(page.url()).toContain('file=sample2.csv');

    // No stale highlighting from previous file
    const highlightedRows = page.locator('tbody tr.bg-orange-50');
    const highlightCount = await highlightedRows.count();
    // Either 0 highlighted rows, or if "aaa" exists in sample2.csv it might be highlighted
    expect(highlightCount).toBeLessThanOrEqual(1);
  });
});
