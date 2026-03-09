import { test, expect } from '@playwright/test';

test.describe('Internationalization (i18n) Support', () => {
  test('should fallback to base column when localization unavailable', async ({ browser }) => {
    // Create a context with a language that has no localization (French)
    const context = await browser.newContext({
      locale: 'fr-FR',
    });
    const page = await context.newPage();

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Since there's no French localization, base columns should be used
    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2);

    // Base column names should be displayed
    await expect(headerCells.nth(0)).toHaveText('question');
    await expect(headerCells.nth(1)).toHaveText('answer');

    // Data should be displayed without errors
    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);

    await context.close();
  });
});
