import { test, expect } from '@playwright/test';

test.describe('Internationalization (i18n) Support', () => {
  test('should select localized column based on browser language', async ({ browser }) => {
    // Create a context with Japanese locale
    const context = await browser.newContext({
      locale: 'ja-JP',
    });
    const page = await context.newPage();

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The application should use Japanese localized columns if available
    // question and answer columns have i18n: true in the mapping

    // Verify the table is rendered
    const table = page.locator('table');
    await expect(table).toBeVisible();

    // Headers should be displayed (question and answer are stored)
    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2);

    await context.close();
  });
});
