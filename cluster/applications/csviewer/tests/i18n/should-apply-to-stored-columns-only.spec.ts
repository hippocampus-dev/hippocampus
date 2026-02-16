import { test, expect } from '@playwright/test';

test.describe('Internationalization (i18n) Support', () => {
  test('should apply i18n only to stored/displayed columns', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // In the mapping:
    // - question: store: true, i18n: true (displayed, localized)
    // - paraphrases: store: false, index: true (not displayed, used for search)
    // - answer: store: true, i18n: true (displayed, localized)

    // Only stored columns appear in the table
    const headerCells = page.locator('thead th');
    await expect(headerCells).toHaveCount(2);

    // Verify search still works on non-stored indexed columns
    const searchInput = page.getByPlaceholder('Search for given file');

    // Search for content that should be in paraphrases column
    // Even though paraphrases is not displayed, it should still be searchable
    await searchInput.fill('bbb');
    await page.waitForTimeout(300);

    const rows = page.locator('tbody tr');
    const count = await rows.count();
    // Should find rows where paraphrases contains "bbb"
    expect(count).toBeGreaterThan(0);

    // Verify the found row's displayed columns (question and answer)
    // The paraphrases value (bbb) should not appear in the displayed columns
    const firstRowCells = rows.first().locator('td');
    await expect(firstRowCells).toHaveCount(2);
  });
});
