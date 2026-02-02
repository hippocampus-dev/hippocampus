import { test, expect } from '@playwright/test';

test.describe('URL-Based State Management and Row Highlighting', () => {
  test('should scroll to highlighted row smoothly', async ({ page }) => {
    // Navigate to the application first to load data
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Get the last row's id (it should be out of viewport initially)
    const lastRow = page.locator('tbody tr').last();
    const lastRowId = await lastRow.getAttribute('id');

    // Navigate with hash pointing to last row
    await page.goto(`/#${lastRowId}`);
    await page.waitForLoadState('networkidle');

    // Wait for scroll animation
    await page.waitForTimeout(1000);

    // The highlighted row should be visible in viewport
    // Use attribute selector to handle special characters in ID
    const highlightedRow = page.locator(`tbody tr[id="${lastRowId}"]`).first();
    await expect(highlightedRow).toBeVisible();
    await expect(highlightedRow).toBeInViewport();
  });
});
