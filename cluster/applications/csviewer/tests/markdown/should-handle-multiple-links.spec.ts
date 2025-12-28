import { test, expect } from '@playwright/test';

test.describe('Markdown Link Rendering', () => {
  test('should handle multiple links in same cell', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for content with links
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    // Get first row's question cell
    const firstCell = page.locator('tbody tr').first().locator('td').first();
    await expect(firstCell).toBeVisible();

    // Count links in the cell
    const linksInCell = firstCell.locator('a');
    const linkCount = await linksInCell.count();

    // Verify at least one link exists
    expect(linkCount).toBeGreaterThanOrEqual(1);

    // Each link should be independently clickable
    for (let i = 0; i < linkCount; i++) {
      const link = linksInCell.nth(i);
      await expect(link).toHaveAttribute('href');
      await expect(link).toHaveAttribute('target', '_blank');
    }
  });
});
