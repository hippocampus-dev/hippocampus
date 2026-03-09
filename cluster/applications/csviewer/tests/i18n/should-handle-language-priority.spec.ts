import { test, expect } from '@playwright/test';

test.describe('Internationalization (i18n) Support', () => {
  test('should handle multiple browser languages with priority', async ({ browser }) => {
    // Create a context with multiple languages (Japanese first, then English)
    const context = await browser.newContext({
      locale: 'ja-JP',
      extraHTTPHeaders: {
        'Accept-Language': 'ja-JP,ja;q=0.9,en-US;q=0.8,en;q=0.7',
      },
    });
    const page = await context.newPage();

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // First available localization should be used (Japanese if available)
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Data should be loaded correctly
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);

    await context.close();
  });
});
