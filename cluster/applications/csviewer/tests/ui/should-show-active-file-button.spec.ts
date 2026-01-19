import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should show active state on selected file button', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Sample1 should be active by default
    const sample1Button = page.getByRole('button', { name: 'Sample1' });
    const sample2Button = page.getByRole('button', { name: 'Sample2' });

    // Active button styling
    await expect(sample1Button).toHaveClass(/text-orange-600/);
    await expect(sample1Button).toHaveClass(/bg-white/);
    await expect(sample1Button).toHaveClass(/ring-4/);
    await expect(sample1Button).toHaveClass(/ring-orange-600/);

    // Inactive button styling
    await expect(sample2Button).toHaveClass(/text-white/);
    await expect(sample2Button).toHaveClass(/bg-orange-600/);

    // Click Sample2 to switch
    await sample2Button.click();
    await page.waitForLoadState('networkidle');

    // Now Sample2 should be active
    await expect(sample2Button).toHaveClass(/text-orange-600/);
    await expect(sample2Button).toHaveClass(/bg-white/);
    await expect(sample2Button).toHaveClass(/ring-4/);

    // And Sample1 should be inactive
    await expect(sample1Button).toHaveClass(/text-white/);
    await expect(sample1Button).toHaveClass(/bg-orange-600/);
  });
});
