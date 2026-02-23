import {test, type Page, type Locator} from "@playwright/test";

test("discover page elements", async ({page}: {page: Page}) => {
    await page.goto("http://localhost:5173");
    await page.waitForLoadState("networkidle");

    const buttons: Locator[] = await page.locator("button").all();
    console.log(`Found ${buttons.length} buttons:`);
    for (let i = 0; i < buttons.length; i++) {
        const button = buttons[i];
        const visible: boolean = await button.isVisible();
        const text: string = visible ? await button.innerText() : "[hidden]";
        console.log(`  [${i}] ${text}`);
    }

    const links: Locator[] = await page.locator("a[href]").all();
    console.log(`\nFound ${links.length} links:`);
    for (const link of links.slice(0, 5)) {
        const text: string = (await link.innerText()).trim();
        const href: string | null = await link.getAttribute("href");
        console.log(`  - ${text} -> ${href}`);
    }

    const inputs: Locator[] = await page.locator("input, textarea, select").all();
    console.log(`\nFound ${inputs.length} input fields:`);
    for (const input of inputs) {
        const name: string = await input.getAttribute("name") || await input.getAttribute("id") || "[unnamed]";
        const inputType: string = await input.getAttribute("type") || "text";
        console.log(`  - ${name} (${inputType})`);
    }

    await page.screenshot({path: "/tmp/page_discovery.png", fullPage: true});
    console.log("\nScreenshot saved to /tmp/page_discovery.png");
});
