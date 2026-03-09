import { test, expect } from '@playwright/test';

test.describe('URL-Based State Management and Row Highlighting', () => {
  test('should preserve file parameter when selecting rows', async ({ page }) => {
    // Navigate to the application with a specific file
    await page.goto('/?file=sample2.csv');
    await page.waitForLoadState('networkidle');

    // Verify file parameter is present
    expect(page.url()).toContain('file=sample2.csv');

    // Click on a row
    const firstRow = page.locator('tbody tr').first();
    await firstRow.click();
    await page.waitForTimeout(300);

    // Both file parameter and hash should be in URL
    const url = page.url();
    expect(url).toContain('file=sample2.csv');
    expect(url).toContain('#');

    // Sample2 button should still be active
    const sample2Button = page.getByRole('button', { name: 'Sample2' });
    await expect(sample2Button).toHaveClass(/bg-white/);
    await expect(sample2Button).toHaveClass(/text-orange-600/);
  });
});
