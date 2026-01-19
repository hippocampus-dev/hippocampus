import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should style search box with focus states', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');
    await expect(searchInput).toBeVisible();

    // Verify search input has proper styling classes
    await expect(searchInput).toHaveClass(/rounded-md/);
    await expect(searchInput).toHaveClass(/border-4/);
    await expect(searchInput).toHaveClass(/border-orange-600/);

    // Verify focus styling classes are defined
    await expect(searchInput).toHaveClass(/focus:ring-4/);
    await expect(searchInput).toHaveClass(/focus:ring-orange-600/);

    // Focus the input
    await searchInput.focus();

    // Input should be focused
    await expect(searchInput).toBeFocused();

    // Verify autofocus attribute
    await expect(searchInput).toHaveAttribute('autofocus', '');
  });
});
