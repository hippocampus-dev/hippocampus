import { parseArgs } from "node:util";
import { chromium } from "playwright-core";

const {
  positionals: [url],
  values: { cdp: cdp },
} = parseArgs({
  options: {
    cdp: {
      type: "string",
      default: "http://127.0.0.1:59222",
    },
  },
  allowPositionals: true,
  strict: true,
});

if (!url) {
  console.error("Usage: node index.mjs [--cdp <url>] <url>");
  process.exit(1);
}

async function testSingleClient(cdp, url, clientId) {
  const browser = await chromium.connectOverCDP(cdp, {
    timeout: 30000,
  });

  try {
    const context = await browser.newContext();

    for (let i = 0; i < 3; i++) {
      const page = await context.newPage();

      const response = await page.goto(url, { timeout: 60000 });
      await page.waitForFunction(
        () =>
          !document.querySelector('meta[name="robots"][content*="noindex"]'),
        { timeout: 30000 },
      );

      console.assert(
        response.status() === 200,
        `client ${clientId} request ${i + 1}: status should be 200, got ${response.status()}`,
      );

      const robots = await page.evaluate(
        () =>
          document
            .querySelector('meta[name="robots"]')
            ?.getAttribute("content") ?? null,
      );
      console.assert(
        robots === null || !robots.includes("noindex"),
        `client ${clientId} request ${i + 1}: robots should not contain noindex`,
      );

      await page.close();
    }

    await context.close();
  } finally {
    await browser.close().catch(() => {});
  }
}

await testSingleClient(cdp, url, 1);

await Promise.all([2, 3, 4].map((id) => testSingleClient(cdp, url, id)));
