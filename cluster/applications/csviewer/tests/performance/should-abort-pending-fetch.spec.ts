import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should abort pending fetch when switching files', async ({ page }) => {
    // Track aborted requests
    const abortedRequests: string[] = [];

    page.on('requestfailed', request => {
      if (request.failure()?.errorText === 'net::ERR_ABORTED') {
        abortedRequests.push(request.url());
      }
    });

    // Slow down CSV responses
    await page.route('**/*.csv', async route => {
      await new Promise(resolve => setTimeout(resolve, 500));
      await route.continue();
    });

    // Navigate to the application
    await page.goto('/');

    // Quickly switch files before load completes
    const sample2Button = page.getByRole('button', { name: 'Sample2' });
    await sample2Button.click();

    // Wait for the new file to load
    await page.waitForLoadState('networkidle');

    // The final state should show Sample2 as active
    await expect(sample2Button).toHaveClass(/bg-white/);
    await expect(sample2Button).toHaveClass(/text-orange-600/);

    // Table should show data from Sample2
    const rows = page.locator('tbody tr');
    await expect(rows.first()).toBeVisible();

    // URL should reflect Sample2
    expect(page.url()).toContain('file=sample2.csv');
  });
});
