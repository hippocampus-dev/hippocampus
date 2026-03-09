import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Basic Search', () => {
  test('should handle multiple search terms separated by spaces', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // Search for single term first
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);
    const singleTermCount = await page.locator('tbody tr').count();

    // Search for multiple terms (AND logic)
    await searchInput.fill('Lorem ipsum');
    await page.waitForTimeout(300);
    const multipleTermCount = await page.locator('tbody tr').count();

    // Multiple terms should filter more (or equal if all match both)
    expect(multipleTermCount).toBeLessThanOrEqual(singleTermCount);

    // Verify all visible rows match ALL terms
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    for (let i = 0; i < Math.min(count, 3); i++) {
      const rowText = await rows.nth(i).textContent();
      const lowerText = rowText?.toLowerCase() || '';
      expect(lowerText).toContain('lorem');
      expect(lowerText).toContain('ipsum');
    }
  });
});
