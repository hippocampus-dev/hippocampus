import { test, expect } from '@playwright/test';

test.describe('Markdown Link Rendering', () => {
  test('should convert markdown links to clickable hyperlinks', async ({ page }) => {
    // Navigate to the application (sample1.csv has markdown links)
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for Lorem to find rows with markdown links
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Find the anchor element rendered from markdown [link is here](https://example.com)
    const link = page.locator('tbody td a[href="https://example.com"]').first();
    await expect(link).toBeVisible();

    // Verify link text
    await expect(link).toHaveText('link is here');

    // Verify href attribute
    await expect(link).toHaveAttribute('href', 'https://example.com');
  });
});
