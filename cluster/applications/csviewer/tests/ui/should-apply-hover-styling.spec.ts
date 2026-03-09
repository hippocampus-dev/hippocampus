import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should apply hover styling on rows', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Get the first row (that is not highlighted)
    const firstRow = page.locator('tbody tr').first();
    await expect(firstRow).toBeVisible();

    // Row should have hover class defined
    await expect(firstRow).toHaveClass(/hover:bg-orange-50/);

    // Row should have cursor-pointer class
    await expect(firstRow).toHaveClass(/cursor-pointer/);

    // Hover over the row
    await firstRow.hover();

    // The hover effect is CSS-based, we can verify the class is present
    // Actual visual change is handled by CSS
    await expect(firstRow).toBeVisible();
  });
});
