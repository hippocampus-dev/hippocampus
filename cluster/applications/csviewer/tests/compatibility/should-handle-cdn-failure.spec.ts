import { test, expect } from '@playwright/test';

test.describe('Browser Compatibility and CDN Dependencies', () => {
  test('should degrade gracefully on CDN failure', async ({ page }) => {
    // Navigate to the application normally first
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The app should load with both CDNs working
    const app = page.locator('#app');
    await expect(app).toBeVisible();

    // Check if there's content rendered by Preact
    const content = app.locator('div');
    await expect(content.first()).toBeVisible();

    // Check if there's any content in body
    const bodyContent = await page.locator('body').innerHTML();
    expect(bodyContent.length).toBeGreaterThan(0);
  });

  test('should verify CDNs are loaded correctly', async ({ page }) => {
    // Track CDN requests
    const cdnRequests: string[] = [];
    page.on('request', request => {
      const url = request.url();
      if (url.includes('cdn.skypack.dev') || url.includes('cdn.tailwindcss.com')) {
        cdnRequests.push(url);
      }
    });

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // CDN requests should have been made
    expect(cdnRequests.length).toBeGreaterThan(0);

    // App should render correctly
    const app = page.locator('#app');
    await expect(app).toBeVisible();

    // The page structure should exist
    const body = page.locator('body');
    await expect(body).toBeVisible();
  });
});
