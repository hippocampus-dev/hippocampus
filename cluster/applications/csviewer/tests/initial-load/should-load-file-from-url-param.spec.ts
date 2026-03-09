import { test, expect } from '@playwright/test';

test.describe('Initial Load and File Selection', () => {
  test('should load specific file when URL contains file parameter', async ({ page }) => {
    // Navigate to the application with ?file=<filename> query parameter
    await page.goto('/?file=sample2.csv');

    // Wait for the page to fully load
    await page.waitForLoadState('networkidle');

    // The specified file is loaded and displayed
    const tableRows = page.locator('tbody tr');
    await expect(tableRows.first()).toBeVisible();

    // The corresponding file button shows active state (Sample2 button)
    const sample2Button = page.getByRole('button', { name: 'Sample2' });
    await expect(sample2Button).toHaveClass(/text-orange-600/);
    await expect(sample2Button).toHaveClass(/bg-white/);
    await expect(sample2Button).toHaveClass(/ring-4/);

    // Sample1 button should NOT have active state (inactive buttons have bg-orange-600, not bg-white)
    const sample1Button = page.getByRole('button', { name: 'Sample1' });
    // Active button has "ring-4 ring-orange-600" (not just "focus:ring-4")
    // Check that sample1 has inactive styling
    const sample1Classes = await sample1Button.getAttribute('class');
    // Inactive button should NOT have "ring-4 ring-orange-600" but SHOULD have "bg-orange-600"
    expect(sample1Classes).toContain('bg-orange-600');
    expect(sample1Classes).not.toMatch(/\sring-4\s/);

    // URL parameter is preserved
    expect(page.url()).toContain('file=sample2.csv');
  });
});
