import { parseArgs } from "node:util";
import { chromium } from "playwright-core";

const {
  positionals: [targetUrl],
  values: { "endpoint-url": endpointUrl },
} = parseArgs({
  options: {
    "endpoint-url": {
      type: "string",
      default: "http://127.0.0.1:59222",
    },
  },
  allowPositionals: true,
  strict: true,
});

if (!targetUrl) {
  console.error("Usage: node index.mjs [--endpoint-url <url>] <target-url>");
  process.exit(1);
}

const browser = await chromium.connectOverCDP(endpointUrl, { timeout: 30000 });

try {
  const context = await browser.newContext();

  for (let i = 0; i < 3; i++) {
    const page = await context.newPage();

    const response = await page.goto(targetUrl, { timeout: 60000 });
    await page.waitForFunction(
      () => !document.querySelector('meta[name="robots"][content*="noindex"]'),
      { timeout: 30000 },
    );

    console.assert(
      response.status() === 200,
      `request ${i + 1}: status should be 200, got ${response.status()}`,
    );

    const robots = await page
      .locator('meta[name="robots"]')
      .getAttribute("content");
    console.assert(
      robots === null || !robots.includes("noindex"),
      `request ${i + 1}: robots should not contain noindex`,
    );

    await page.close();
  }

  await context.close();
} finally {
  await browser.close().catch(() => {});
}
