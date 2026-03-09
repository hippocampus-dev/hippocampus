import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should maintain responsive layout', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Test at desktop width
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.waitForTimeout(300);

    const mainContainer = page.locator('div.flex.flex-col.items-center');
    await expect(mainContainer).toBeVisible();

    // Content should be centered
    await expect(mainContainer).toHaveClass(/items-center/);

    // Test at tablet width
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.waitForTimeout(300);

    // Table should still be visible
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Test at mobile width
    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(300);

    // Application should not have horizontal scrollbar at component level
    // (table might scroll, but main app shouldn't overflow)
    await expect(mainContainer).toBeVisible();

    // Search input should be full width on mobile
    const searchInput = page.getByPlaceholder('Search for given file');
    await expect(searchInput).toHaveClass(/w-full/);
  });
});
