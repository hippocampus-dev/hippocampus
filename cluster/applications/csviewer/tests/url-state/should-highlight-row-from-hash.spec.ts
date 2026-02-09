import { test, expect } from '@playwright/test';

test.describe('URL-Based State Management and Row Highlighting', () => {
  test('should highlight row when URL contains hash', async ({ page }) => {
    // Navigate to the application with a hash (using URL-encoded value)
    // The pkey is "question" column, so hash should match a question value
    await page.goto('/#aaa');
    await page.waitForLoadState('networkidle');

    // Wait for the hash to be processed and row to be highlighted
    await page.waitForTimeout(500);

    // Find rows with id="aaa" (there may be multiple due to duplicate data)
    const highlightedRows = page.locator('tbody tr#aaa');
    const count = await highlightedRows.count();
    expect(count).toBeGreaterThan(0);

    // Verify at least one row has highlight styling (bg-orange-50)
    const firstHighlighted = highlightedRows.first();
    await expect(firstHighlighted).toBeVisible();
    await expect(firstHighlighted).toHaveClass(/bg-orange-50/);
  });
});
