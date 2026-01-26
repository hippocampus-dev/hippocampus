import { test, expect } from '@playwright/test';

test.describe('Initial Load and File Selection', () => {
  test('should switch files when clicking file buttons', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');

    // Wait for initial file to load
    await page.waitForLoadState('networkidle');

    // Verify initial state - Sample1 should be active
    const sample1Button = page.getByRole('button', { name: 'Sample1' });
    const sample2Button = page.getByRole('button', { name: 'Sample2' });

    await expect(sample1Button).toHaveClass(/text-orange-600/);
    await expect(sample1Button).toHaveClass(/bg-white/);

    // Click on a different file button
    await sample2Button.click();

    // Wait for new file to load
    await page.waitForLoadState('networkidle');

    // New file data is loaded and displayed
    const tableRows = page.locator('tbody tr');
    await expect(tableRows.first()).toBeVisible();

    // URL is updated with new file parameter
    await page.waitForURL(/file=sample2\.csv/, { timeout: 5000 });

    // Previous file button loses active state (should have bg-orange-600 for inactive)
    const sample1Classes = await sample1Button.getAttribute('class');
    expect(sample1Classes).toContain('bg-orange-600');
    // Active state has "bg-white" (not focus:bg-white), check it's not there
    // The inactive button should have bg-orange-600 which means it's not active
    expect(sample1Classes).not.toMatch(/\sbg-white\s/);

    // Clicked button gains active state
    await expect(sample2Button).toHaveClass(/text-orange-600/);
    await expect(sample2Button).toHaveClass(/bg-white/);
    await expect(sample2Button).toHaveClass(/ring-4/);
  });
});
