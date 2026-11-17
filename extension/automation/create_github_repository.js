export default {
  name: "Create GitHub repository",
  columns: ["repository"],
  optionalColumns: ["description", "visibility"],
  resultColumns: ["html_url"],
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
};
