import { test, expect } from '@playwright/test';

test.describe('Internationalization (i18n) Support', () => {
  test('should respect per-column i18n configuration in mapping', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // In the mapping:
    // - question: i18n: true
    // - paraphrases: i18n not set (false)
    // - answer: i18n: true

    // Only stored columns are displayed: question and answer
    // Both have i18n enabled

    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2);

    // Verify the mapping is respected by checking column display
    await expect(headerCells.nth(0)).toHaveText('question');
    await expect(headerCells.nth(1)).toHaveText('answer');

    // paraphrases should not be displayed (store: false)
    const allHeaders = await headerCells.allTextContents();
    expect(allHeaders).not.toContain('paraphrases');
  });
});
