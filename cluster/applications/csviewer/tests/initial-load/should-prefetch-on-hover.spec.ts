import { test, expect } from '@playwright/test';

test.describe('Initial Load and File Selection', () => {
  test('should prefetch file on button hover', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');

    // Wait for the page to fully load
    await page.waitForLoadState('networkidle');

    // Verify no prefetch link exists initially for sample2.csv
    let prefetchLink = page.locator('head link[rel="prefetch"][href="sample2.csv"]');
    await expect(prefetchLink).toHaveCount(0);

    // Hover over a file button (not the currently active one)
    const sample2Button = page.getByRole('button', { name: 'Sample2' });
    await sample2Button.hover();

    // Wait a moment for the prefetch link to be added
    await page.waitForTimeout(100);

    // Prefetch link should be added to head
    prefetchLink = page.locator('head link[rel="prefetch"][href="sample2.csv"]');
    await expect(prefetchLink).toHaveCount(1);

    // Verify the prefetch link has correct attributes
    await expect(prefetchLink).toHaveAttribute('rel', 'prefetch');
    await expect(prefetchLink).toHaveAttribute('href', 'sample2.csv');
  });
});
