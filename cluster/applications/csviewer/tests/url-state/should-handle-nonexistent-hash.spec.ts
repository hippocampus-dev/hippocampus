import { test, expect } from '@playwright/test';

test.describe('URL-Based State Management and Row Highlighting', () => {
  test('should handle non-existent hash gracefully', async ({ page }) => {
    // Listen for console errors
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    // Navigate to the application with a hash that doesn't match any row
    await page.goto('/#nonexistent-row-id-12345');
    await page.waitForLoadState('networkidle');

    // Application should not crash
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // All rows should be visible (not filtered)
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);

    // No row should be highlighted (since the hash doesn't match)
    const highlightedRows = page.locator('tbody tr.bg-orange-50');
    await expect(highlightedRows).toHaveCount(0);

    // No critical errors should be logged
    const criticalErrors = consoleErrors.filter(e =>
      e.includes('Cannot read') || e.includes('undefined') || e.includes('null')
    );
    expect(criticalErrors.length).toBe(0);

    // Search should still work
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('aaa');
    await page.waitForTimeout(300);
    const filteredCount = await page.locator('tbody tr').count();
    expect(filteredCount).toBeGreaterThan(0);
  });
});
