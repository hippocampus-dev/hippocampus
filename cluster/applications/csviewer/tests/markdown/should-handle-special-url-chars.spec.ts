import { test, expect } from '@playwright/test';

test.describe('Markdown Link Rendering', () => {
  test('should handle special characters in URLs', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for content with links
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Find all links
    const links = page.locator('tbody td a');
    const linkCount = await links.count();
    expect(linkCount).toBeGreaterThan(0);

    // Verify each link has a valid href
    for (let i = 0; i < Math.min(linkCount, 5); i++) {
      const link = links.nth(i);
      const href = await link.getAttribute('href');

      // href should be a valid URL
      expect(href).toBeTruthy();
      expect(href).toMatch(/^https?:\/\//);
    }
  });
});
