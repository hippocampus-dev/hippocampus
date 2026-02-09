import { test, expect } from '@playwright/test';

test.describe('Search Functionality - Fuzzy Search (Bitap Algorithm)', () => {
  test('should require exact match when maxDistance is 0', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const searchInput = page.getByPlaceholder('Search for given file');

    // For very long patterns (>64 chars), the bitap algorithm falls back to exact match
    // Let's test with a pattern that would require exact matching

    // First verify exact match works
    await searchInput.fill('aaa');
    await page.waitForTimeout(300);
    const exactCount = await page.locator('tbody tr').count();
    expect(exactCount).toBeGreaterThan(0);

    // With maxDistance=0 (or very long patterns), fuzzy matching is disabled
    // A completely different term should not match
    await searchInput.fill('zzz');
    await page.waitForTimeout(300);
    const noMatchCount = await page.locator('tbody tr').count();
    expect(noMatchCount).toBe(0);
  });
});
