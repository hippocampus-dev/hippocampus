import {test, type Page} from "@playwright/test";
import {resolve} from "node:path";

test("automate static HTML file", async ({page}: {page: Page}) => {
    const htmlFilePath: string = resolve("path/to/your/file.html");
    const fileUrl = `file://${htmlFilePath}`;

    await page.goto(fileUrl);
    await page.screenshot({path: "/tmp/static_page.png", fullPage: true});

    await page.click("text=Click Me");
    await page.fill("#name", "John Doe");
    await page.fill("#email", "john@example.com");

    await page.click('button[type="submit"]');
    await page.waitForTimeout(500);

    await page.screenshot({path: "/tmp/after_submit.png", fullPage: true});

    console.log("Static HTML automation completed!");
});
