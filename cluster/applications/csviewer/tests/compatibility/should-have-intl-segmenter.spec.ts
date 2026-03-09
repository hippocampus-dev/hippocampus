import { test, expect } from '@playwright/test';

test.describe('Browser Compatibility and CDN Dependencies', () => {
  test('should verify Intl.Segmenter API availability', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Check if Intl.Segmenter is available
    const segmenterAvailable = await page.evaluate(() => {
      return typeof Intl.Segmenter !== 'undefined';
    });

    // Intl.Segmenter should be available in modern browsers
    expect(segmenterAvailable).toBe(true);

    // Test that segmentation works for Japanese text
    const segmenterWorks = await page.evaluate(() => {
      try {
        const segmenter = new Intl.Segmenter('ja');
        const segments = [...segmenter.segment('テスト')];
        return segments.length > 0;
      } catch (e) {
        return false;
      }
    });

    expect(segmenterWorks).toBe(true);

    // Search should work (which uses Intl.Segmenter internally)
    const searchInput = page.getByPlaceholder('Search for given file');
    await searchInput.fill('Lorem');
    await page.waitForTimeout(300);

    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);
  });
});
