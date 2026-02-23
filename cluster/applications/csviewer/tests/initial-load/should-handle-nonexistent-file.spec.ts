import { test, expect } from '@playwright/test';

test.describe('Initial Load and File Selection', () => {
  test('should handle non-existent file parameter gracefully', async ({ page }) => {
    // Navigate to the application with ?file=nonexistent query parameter
    await page.goto('/?file=nonexistent.csv');

    // Wait for the page to attempt loading
    await page.waitForLoadState('networkidle');

    // Application does not crash - page should still be functional
    const fileButtons = page.locator('button');
    await expect(fileButtons.first()).toBeVisible();

    // The default file (sample1.csv) should be shown as active since nonexistent file is not in the files object
    const sample1Button = page.getByRole('button', { name: 'Sample1' });
    await expect(sample1Button).toHaveClass(/text-orange-600/);
    await expect(sample1Button).toHaveClass(/bg-white/);

    // Search input should still be functional
    const searchInput = page.getByPlaceholder('Search for given file');
    await expect(searchInput).toBeVisible();
    await expect(searchInput).toBeEnabled();
  });
});
