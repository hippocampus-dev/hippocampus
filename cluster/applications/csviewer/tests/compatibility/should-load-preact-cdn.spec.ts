import { test, expect } from '@playwright/test';

test.describe('Browser Compatibility and CDN Dependencies', () => {
  test('should load Preact from CDN successfully', async ({ page }) => {
    // Track network requests
    const cdnRequests: string[] = [];
    page.on('request', request => {
      if (request.url().includes('cdn.skypack.dev')) {
        cdnRequests.push(request.url());
      }
    });

    // Track successful responses
    const successfulResponses: string[] = [];
    page.on('response', response => {
      if (response.url().includes('cdn.skypack.dev') && response.ok()) {
        successfulResponses.push(response.url());
      }
    });

    // Navigate to the application
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Verify Preact was requested from CDN
    const preactRequests = cdnRequests.filter(url => url.includes('preact'));
    expect(preactRequests.length).toBeGreaterThan(0);

    // Verify Preact loaded successfully
    const preactResponses = successfulResponses.filter(url => url.includes('preact'));
    expect(preactResponses.length).toBeGreaterThan(0);

    // Verify the application renders (Preact is working)
    const app = page.locator('#app');
    await expect(app).toBeVisible();

    // Child elements should be rendered by Preact
    const content = app.locator('div');
    await expect(content.first()).toBeVisible();
  });
});
