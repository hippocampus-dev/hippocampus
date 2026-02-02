import {test, type Page, type ConsoleMessage} from "@playwright/test";
import {writeFileSync} from "node:fs";

test("capture console logs", async ({page}: {page: Page}) => {
    const consoleLogs: string[] = [];

    page.on("console", (message: ConsoleMessage) => {
        const logEntry = `[${message.type()}] ${message.text()}`;
        consoleLogs.push(logEntry);
        console.log(`Console: ${logEntry}`);
    });

    await page.goto("http://localhost:5173");
    await page.waitForLoadState("networkidle");

    await page.click("text=Dashboard");
    await page.waitForTimeout(1000);

    writeFileSync("/tmp/console.log", consoleLogs.join("\n"));

    console.log(`\nCaptured ${consoleLogs.length} console messages`);
    console.log("Logs saved to: /tmp/console.log");
});
