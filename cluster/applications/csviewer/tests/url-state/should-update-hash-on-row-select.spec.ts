import { test, expect } from '@playwright/test';

test.describe('URL-Based State Management and Row Highlighting', () => {
  test('should update URL hash when row is selected', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Get initial URL (should have no hash)
    const initialUrl = page.url();
    expect(initialUrl).not.toContain('#');

    // Click on a row
    const firstRow = page.locator('tbody tr').first();
    await firstRow.click();

    // Wait for hash update
    await page.waitForTimeout(300);

    // URL should now contain a hash
    const newUrl = page.url();
    expect(newUrl).toContain('#');

    // The row should be highlighted
    await expect(firstRow).toHaveClass(/bg-orange-50/);
  });
});
