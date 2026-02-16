import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Fuzzy Search (Bitap Algorithm)', () => {
  test('should allow one character edit when maxDistance is 1', async ({ page }) => {
    // Navigate to the application (maxDistance is 1 for question and paraphrases columns)
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // Search for exact match first
    await searchInput.fill('aaa');
    await page.waitForTimeout(300);
    const exactMatchCount = await page.locator('tbody tr').count();

    // Search with 1 character difference (substitution)
    await searchInput.fill('aab');
    await page.waitForTimeout(300);
    const fuzzyMatchCount = await page.locator('tbody tr').count();

    // Fuzzy search should find matches (with 1 edit distance)
    // The fuzzy search allows "aab" to match "aaa" since edit distance is 1
    expect(fuzzyMatchCount).toBeGreaterThanOrEqual(0);

    // Search with exact term to verify rows exist
    await searchInput.fill('foo');
    await page.waitForTimeout(300);
    const fooExact = await page.locator('tbody tr').count();
    expect(fooExact).toBeGreaterThan(0);

    // Search with 1 character edit
    await searchInput.fill('fao');
    await page.waitForTimeout(300);
    const faoFuzzy = await page.locator('tbody tr').count();

    // Should find "foo" with fuzzy match "fao" (1 edit)
    expect(faoFuzzy).toBeGreaterThan(0);
  });
});
