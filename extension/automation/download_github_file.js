export default {
  name: "Download GitHub file",
  columns: ["url"],
  optionalColumns: ["filename"],
  resultColumns: ["path"],
  beforeAll: async (page) => {
    // Signed-out users are redirected to /login?return_to=https%3A%2F%2Fgithub.com%2Fsettings%2Fprofile
    await page.goto("https://github.com/settings/profile");
    await page.waitForURL((url) => url.pathname === "/settings/profile");
  },
  run: async (page, row) => {
    await page.goto(row.url);
    const downloadPromise = page.waitForEvent("download");
    await page.getByLabel("Download raw file", { exact: true }).click();
    const download = await downloadPromise;
    const filename = row.filename?.trim();
    if (filename) {
      await download.saveAs(filename);
    }
    return { path: await download.path() };
  },
};
