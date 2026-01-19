import { test, expect } from '@playwright/test';

test.describe('Markdown Link Rendering', () => {
  test('should open links in new tab', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for content with markdown links
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Find the anchor element
    const link = page.locator('tbody td a[href="https://example.com"]').first();
    await expect(link).toBeVisible();

    // Verify target="_blank" attribute for opening in new tab
    await expect(link).toHaveAttribute('target', '_blank');
  });
});
