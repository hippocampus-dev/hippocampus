import { test, expect } from '@playwright/test';

test.describe('Markdown Link Rendering', () => {
  test('should handle malformed markdown gracefully', async ({ page }) => {
    // Listen for console errors
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Application should load without errors
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // All rows should be visible
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);

    // No console errors related to markdown parsing
    const markdownErrors = consoleErrors.filter(e =>
      e.toLowerCase().includes('markdown') || e.includes('link')
    );
    expect(markdownErrors.length).toBe(0);

    // Cells should have content (not broken)
    const firstCell = rows.first().locator('td').first();
    const cellContent = await firstCell.textContent();
    expect(cellContent?.length).toBeGreaterThan(0);
  });
});
