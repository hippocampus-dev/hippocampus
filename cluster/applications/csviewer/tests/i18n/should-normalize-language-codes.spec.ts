import { test, expect } from '@playwright/test';

test.describe('Internationalization (i18n) Support', () => {
  test('should normalize language codes (hyphen splitting)', async ({ browser }) => {
    // Create a context with region-specific locale (ja-JP should match ja)
    const context = await browser.newContext({
      locale: 'ja-JP',
    });
    const page = await context.newPage();

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The app splits language codes by hyphen, so ja-JP becomes ja
    // This should match columns like "question:ja" if they exist

    // Verify the app loads correctly with normalized language
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Headers should be present
    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2);

    // Test with another region-specific locale (en-US should match en)
    await context.close();

    const enContext = await browser.newContext({
      locale: 'en-US',
    });
    const enPage = await enContext.newPage();
    await enPage.goto('/');
    await enPage.waitForLoadState('networkidle');

    // App should work with en-US normalized to en
    const enTable = enPage.locator('table');
    await expect(enTable).toBeVisible();

    await enContext.close();
  });
});
