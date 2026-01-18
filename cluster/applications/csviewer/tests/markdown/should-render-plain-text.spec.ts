import { test, expect } from '@playwright/test';

test.describe('Markdown Link Rendering', () => {
  test('should render plain text without markdown unchanged', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Search for rows with plain text (no markdown links)
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('aaa');
    await page.waitForTimeout(300);

    // Get the first row's cell
    const firstCell = page.locator('tbody tr').first().locator('td').first();
    await expect(firstCell).toBeVisible();

    // Verify plain text is displayed as-is
    await expect(firstCell).toHaveText('aaa');

    // No anchor tags should be present for plain text
    const links = firstCell.locator('a');
    await expect(links).toHaveCount(0);
  });
});
