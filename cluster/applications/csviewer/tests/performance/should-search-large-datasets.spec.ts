import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should search large datasets efficiently', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // Measure search time
    const startTime = Date.now();
    await searchInput.fill('Lorem');
    await page.waitForTimeout(500); // Allow for debounce/search
    const searchTime = Date.now() - startTime;

    // Search should be fast (under 1 second including debounce)
    expect(searchTime).toBeLessThan(1000);

    // Results should be displayed
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);

    // UI should remain responsive during search
    await expect(searchInput).toBeEnabled();
    await expect(searchInput).toHaveValue('Lorem');
  });
});
