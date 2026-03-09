import { test, expect } from '@playwright/test';

test.describe('Performance and Edge Cases', () => {
  test('should manage memory on repeated file switches', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const sample1Button = page.getByRole('button', { name: 'Sample1' });
    const sample2Button = page.getByRole('button', { name: 'Sample2' });

    // Switch files multiple times
    for (let i = 0; i < 5; i++) {
      await sample2Button.click();
      await page.waitForLoadState('networkidle');

      await sample1Button.click();
      await page.waitForLoadState('networkidle');
    }

    // Application should still be responsive
    const searchInput = page.getByPlaceholder('Search for given file');
    await expect(searchInput).toBeEnabled();

    // Search should still work
    await searchInput.fill('aaa');
    await page.waitForTimeout(300);

    const rows = page.locator('tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);

    // Clear search
    await searchInput.fill('');
    await page.waitForTimeout(300);

    // All rows should be visible again
    const allRows = await page.locator('tbody tr').count();
    expect(allRows).toBeGreaterThan(count);
  });
});
