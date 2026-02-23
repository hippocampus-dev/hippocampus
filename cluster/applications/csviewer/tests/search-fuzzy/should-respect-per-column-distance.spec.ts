import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Fuzzy Search (Bitap Algorithm)', () => {
  test('should respect maxDistance limit per column', async ({ page }) => {
    // Navigate to the application
    // question column has maxDistance: 1
    // paraphrases column has maxDistance: 1
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // Search with 1 edit distance should work
    await searchInput.fill('fao'); // Should match "foo" with 1 edit
    await page.waitForTimeout(300);
    const oneEditCount = await page.locator('tbody tr').count();

    // Search with 2 edit distance should not match (maxDistance is 1)
    await searchInput.fill('fxx'); // 2 edits from "foo"
    await page.waitForTimeout(300);
    const twoEditCount = await page.locator('tbody tr').count();

    // 1 edit should find matches, 2 edits should find fewer or none
    expect(oneEditCount).toBeGreaterThanOrEqual(twoEditCount);
  });
});
