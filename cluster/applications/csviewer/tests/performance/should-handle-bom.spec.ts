import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should handle BOM (Byte Order Mark) in CSV files', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Get the first header cell
    const firstHeader = page.locator('thead th').first();
    await expect(firstHeader).toBeVisible();

    // Header should not have BOM character (U+FEFF)
    const headerText = await firstHeader.textContent();
    expect(headerText).not.toContain('\uFEFF');
    expect(headerText).toBe('question');

    // First data cell should also be clean
    const firstCell = page.locator('tbody tr').first().locator('td').first();
    const cellText = await firstCell.textContent();
    expect(cellText).not.toContain('\uFEFF');
  });
});
