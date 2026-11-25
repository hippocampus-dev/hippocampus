import { HOST_NAME } from "../lib/native_messaging.js";

export default {
  name: "Create GitHub repository",
  columns: ["repository"],
  optionalColumns: ["description", "visibility"],
  resultColumns: ["html_url"],
  // https://playwright.dev/docs/test-timeouts
  timeout: 30000,
  beforeAll: async (page) => {
    // Signed-out users are redirected to /login?return_to=https%3A%2F%2Fgithub.com%2Fsettings%2Fprofile
    await page.goto("https://github.com/settings/profile");
    await page.waitForURL((url) => url.pathname === "/settings/profile");
  },
  run: async (page, row) => {
    const visibility = row.visibility || "private";
    if (!["public", "private"].includes(visibility)) {
      throw new Error(`visibility must be public or private: ${visibility}`);
    }
    // https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository#creating-a-new-repository-from-a-url-query
    const parameters = new URLSearchParams({
      name: row.repository,
      description: row.description ?? "",
      visibility,
    });
    await page.goto(`https://github.com/new?owner=@me&${parameters}`);
    await page.locator("main button[type=submit]").click();
    await page.waitForURL((url) => url.pathname.endsWith(`/${row.repository}`));
    return { html_url: page.url() };
  },
  afterAll: async (_page, { fileName, records, results, csv }) => {
    // https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging
    await chrome.runtime.sendNativeMessage(HOST_NAME, {
      type: "create_github_repository",
      fileName,
      records,
      results,
      csv,
    });
  },
};
