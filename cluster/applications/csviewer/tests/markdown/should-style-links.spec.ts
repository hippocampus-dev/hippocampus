import { test, expect } from '@playwright/test';

test.describe('Markdown Link Rendering', () => {
  test('should style links appropriately', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for content with links
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Find a link
    const link = page.locator('tbody td a').first();
    await expect(link).toBeVisible();

    // Verify link has underline styling
    await expect(link).toHaveClass(/underline/);

    // Verify link has hover styles defined
    await expect(link).toHaveClass(/hover:text-orange-800/);

    // Verify link has focus styles for accessibility
    await expect(link).toHaveClass(/focus:outline-none/);
    await expect(link).toHaveClass(/focus:text-orange-800/);
  });
});
