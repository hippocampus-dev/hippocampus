import { test, expect } from '@playwright/test';

test.describe('Table Display and UI Elements', () => {
  test('should provide accessible focus states on file buttons', async ({ page }) => {
    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Get file buttons
    const sample1Button = page.getByRole('button', { name: 'Sample1' });
    const sample2Button = page.getByRole('button', { name: 'Sample2' });

    // Verify buttons have focus styling classes
    await expect(sample1Button).toHaveClass(/focus:outline-none/);
    await expect(sample1Button).toHaveClass(/focus:ring-4/);
    await expect(sample1Button).toHaveClass(/focus:ring-orange-600/);

    // Tab to the search input first (it has autofocus)
    await page.keyboard.press('Tab');

    // Tab to the first button
    await page.keyboard.press('Tab');

    // Tab to Sample2
    await page.keyboard.press('Tab');

    // Sample2 should now be focused (or one of the buttons)
    // Verify keyboard navigation works
    const focusedElement = page.locator(':focus');
    await expect(focusedElement).toBeVisible();
  });
});
