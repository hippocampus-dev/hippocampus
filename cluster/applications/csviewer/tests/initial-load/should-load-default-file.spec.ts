import { test, expect } from '@playwright/test';

test.describe('Initial Load and File Selection', () => {
  test('should load default file on initial page visit', async ({ page }) => {
    // Navigate to the application root URL without any query parameters
    await page.goto('/');

    // Wait for the page to fully load
    await page.waitForLoadState('networkidle');

    // The first file defined in the mapping configuration is loaded (sample1.csv with alias "Sample1")
    const sample1Button = page.getByRole('button', { name: 'Sample1' });
    await expect(sample1Button).toBeVisible();

    // The corresponding file button shows active state styling (text-orange-600 bg-white ring-4)
    await expect(sample1Button).toHaveClass(/text-orange-600/);
    await expect(sample1Button).toHaveClass(/bg-white/);
    await expect(sample1Button).toHaveClass(/ring-4/);

    // CSV data is displayed in the table
    const tableRows = page.locator('tbody tr');
    await expect(tableRows.first()).toBeVisible();
    const rowCount = await tableRows.count();
    expect(rowCount).toBeGreaterThan(0);
  });
});
