import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import {
  type Dialog,
  type Download,
  expect,
  type Page as PlaywrightPage,
  test,
} from "@playwright/test";

type Expect = typeof expect;

const HTML = `
<title>Main</title>
<script>
window.events = [];
for (const type of ["click", "dblclick", "contextmenu", "mouseover", "input", "change", "keydown"]) {
  document.addEventListener(type, (event) => {
    events.push([type, event.target.id || event.target.localName, event.key ?? event.button, event.shiftKey].join(" "));
  }, true);
}
window.later = () => setTimeout(() => {
  document.getElementById("later").append(Object.assign(document.createElement("span"), { id: "added", textContent: "Added" }));
  document.getElementById("removed").remove();
}, 300);
</script>
<button>Save</button>
<div>Save <b>draft</b></div>
<span style="display:none">Save</span>
<input type="submit" value="Go">
<input type="button" value="Run">
<input type="reset" value="Clear">
<div id="script">Code<script>1</script></div>
<div id="svg"><svg><style>.a{}</style></svg>Icon</div>
<div id="shadow"><template shadowrootmode="open">Icon<slot></slot></template>Label</div>
<label>Pick<script>1</script><input type="submit" value="Now"></label>
<label>Name <input value="x"></label>
<span id="mail">Mail</span><input aria-labelledby="mail">
<label for="check">Check</label><input id="check" type="checkbox" checked>
<input id="radio" type="radio">
<div role="checkbox" aria-checked="mixed">Mixed</div>
<p>a&nbsp;b</p>
<input placeholder="Search here">
<div data-testid="panel">Panel</div>
<a href="#target" title="Link title">Link</a>
<a id="other" href="/other">Other</a>
<a id="download" href="/download">Download</a>
<a id="download-other" href="/download-other">Download other</a>
<a id="download-redirect" href="/download-redirect">Download redirect</a>
<input id="text" value="initial">
<input id="readonly" value="fixed" readonly>
<textarea id="area"></textarea>
<div id="editable" contenteditable>Edit</div>
<button id="disabled" disabled>Off</button>
<select id="single"><option value="a">Alpha</option><option value="b">Beta</option></select>
<select id="multiple" multiple><option value="1">One</option><option value="2">Two</option><option value="3">Three</option></select>
<div id="later"></div>
<div id="removed">Removed</div>
`;

const PAGES: Record<string, [string, string]> = {
  "/": ["text/html", HTML],
  "/other": ["text/html", "<title>Other</title><p>Other page</p>"],
  "/data": ["application/json", '{"answer":42}'],
};

const CASES: [
  string,
  (page: PlaywrightPage, assert: Expect, origin: string) => Promise<unknown>,
][] = [
  ["getByText substring", (page) => page.getByText("Save").count()],
  [
    "getByText exact",
    (page) => page.getByText("Save", { exact: true }).count(),
  ],
  ["getByText regular expression", (page) => page.getByText(/draft/).count()],
  ["getByText submit input", (page) => page.getByText("Go").count()],
  [
    "getByText button input",
    (page) => page.getByText("Run", { exact: true }).count(),
  ],
  ["getByText reset input", (page) => page.getByText("Clear").count()],
  [
    "getByText non-breaking space",
    (page) => page.getByText("a b", { exact: true }).count(),
  ],
  ["getByLabel wrapping label", (page) => page.getByLabel("Name").inputValue()],
  ["getByLabel aria-labelledby", (page) => page.getByLabel("Mail").count()],
  ["getByLabel checkbox", (page) => page.getByLabel("Check").isChecked()],
  [
    "getByLabel label text",
    (page) => page.getByLabel("PickNow", { exact: true }).count(),
  ],
  ["getByPlaceholder", (page) => page.getByPlaceholder("search").count()],
  ["getByTestId", (page) => page.getByTestId("panel").textContent()],
  ["css engine", (page) => page.locator("css=button").count()],
  ["xpath", (page) => page.locator("//select/option").count()],
  ["xpath engine", (page) => page.locator("xpath=//b").textContent()],
  ["nested locator", (page) => page.locator("div").locator("b").textContent()],
  ["aria-checked mixed", (page) => page.locator("[role=checkbox]").isChecked()],
  [
    "filter hasText",
    (page) => page.locator("div").filter({ hasText: "draft" }).innerText(),
  ],
  [
    "filter hasNotText",
    (page) => page.locator("div").filter({ hasNotText: "Save" }).count(),
  ],
  [
    "filter has",
    (page) =>
      page
        .locator("label")
        .filter({ has: page.locator("input") })
        .count(),
  ],
  [
    "filter hasNot",
    (page) =>
      page
        .locator("div")
        .filter({ hasNot: page.locator("b") })
        .count(),
  ],
  [
    "filter visible",
    (page) => page.getByText("Save").filter({ visible: false }).count(),
  ],
  ["nth", (page) => page.locator("option").nth(2).textContent()],
  ["first", (page) => page.getByText("Save").first().textContent()],
  ["last", (page) => page.getByText("Save").last().textContent()],
  ["isVisible", (page) => page.getByText("Save").first().isVisible()],
  ["isVisible hidden", (page) => page.getByText("Save").nth(2).isVisible()],
  ["isVisible missing", (page) => page.locator("table").isVisible()],
  ["getAttribute", (page) => page.getByText("Link").getAttribute("title")],
  [
    "getAttribute missing",
    (page) => page.getByText("Link").getAttribute("lang"),
  ],
  ["innerText", (page) => page.locator("#shadow").innerText()],
  ["inputValue select", (page) => page.locator("#single").inputValue()],
  [
    "inputValue not an input",
    (page) => page.locator("#editable").inputValue({ timeout: 1000 }),
  ],
  ["isChecked radio", (page) => page.locator("#radio").isChecked()],
  [
    "isChecked not a checkbox",
    (page) => page.locator("#text").isChecked({ timeout: 1000 }),
  ],
  [
    "strict mode violation",
    (page) => page.getByText("Save").textContent({ timeout: 1000 }),
  ],
  [
    "missing element",
    (page) => page.locator("table").textContent({ timeout: 1000 }),
  ],
  ["malformed selector", (page) => page.locator("div[").count()],
  [
    "click",
    async (page) => {
      await page.locator("#other").click();
      await page.waitForURL("**/other");
      return page.url();
    },
  ],
  [
    "click events",
    async (page) => {
      await page.getByText("Save", { exact: true }).click();
      return page.evaluate(() => window.events);
    },
  ],
  [
    "click right button with modifier",
    async (page) => {
      await page
        .getByText("Save", { exact: true })
        .click({ button: "right", modifiers: ["Shift"] });
      return page.evaluate(() => window.events);
    },
  ],
  [
    "click position and clickCount",
    async (page) => {
      await page
        .getByText("Save", { exact: true })
        .click({ position: { x: 1, y: 1 }, clickCount: 2 });
      return page.evaluate(() => window.events);
    },
  ],
  [
    "click disabled",
    (page) => page.locator("#disabled").click({ timeout: 1000 }),
  ],
  [
    "click hidden",
    (page) => page.getByText("Save").nth(2).click({ timeout: 1000 }),
  ],
  [
    "click force",
    async (page) => {
      await page.locator("#disabled").click({ force: true, timeout: 1000 });
      return page.evaluate(() => window.events);
    },
  ],
  [
    "dblclick",
    async (page) => {
      await page.getByText("Save", { exact: true }).dblclick();
      return page.evaluate(() => window.events);
    },
  ],
  [
    "hover",
    async (page) => {
      await page.getByText("Save", { exact: true }).hover();
      return page.evaluate(() => window.events);
    },
  ],
  [
    "fill input",
    async (page) => {
      await page.locator("#text").fill("hello");
      return [
        await page.locator("#text").inputValue(),
        await page.evaluate(() => window.events),
      ];
    },
  ],
  [
    "fill empty",
    async (page) => {
      await page.locator("#text").fill("");
      return page.locator("#text").inputValue();
    },
  ],
  [
    "fill textarea",
    async (page) => {
      await page.locator("#area").fill("line\nbreak");
      return page.locator("#area").inputValue();
    },
  ],
  [
    "fill contenteditable",
    async (page) => {
      await page.locator("#editable").fill("changed");
      return page.locator("#editable").textContent();
    },
  ],
  [
    "fill readonly",
    (page) => page.locator("#readonly").fill("x", { timeout: 1000 }),
  ],
  [
    "fill not editable",
    (page) => page.getByTestId("panel").fill("x", { timeout: 1000 }),
  ],
  [
    "press",
    async (page) => {
      await page.locator("#text").press("Backspace");
      return [
        await page.locator("#text").inputValue(),
        await page.evaluate(() => window.events),
      ];
    },
  ],
  [
    "press with modifier",
    async (page) => {
      await page.locator("#area").press("Shift+KeyA");
      return [
        await page.locator("#area").inputValue(),
        await page.evaluate(() => window.events),
      ];
    },
  ],
  [
    "press Shift with characters",
    async (page) => {
      await page.locator("#area").press("Shift+a");
      await page.locator("#area").press("Shift+1");
      await page.locator("#area").press("Shift+Enter");
      return [
        await page.locator("#area").inputValue(),
        await page.evaluate(() => window.events),
      ];
    },
  ],
  [
    "press shifted symbols",
    async (page) => {
      await page.locator("#area").press("Shift+Digit1");
      await page.locator("#area").press("Shift+Quote");
      return page.locator("#area").inputValue();
    },
  ],
  [
    "press ControlOrMeta",
    async (page) => {
      await page.locator("#text").press("ControlOrMeta+a");
      await page.keyboard.press("Delete");
      return page.locator("#text").inputValue();
    },
  ],
  [
    "press Enter",
    async (page) => {
      await page.locator("#area").press("Enter");
      return page.locator("#area").inputValue();
    },
  ],
  [
    "pressSequentially",
    async (page) => {
      await page.locator("#area").pressSequentially("Hi, there! 1+1=2");
      return page.locator("#area").inputValue();
    },
  ],
  [
    "keyboard type",
    async (page) => {
      await page.locator("#area").click();
      await page.keyboard.type("typed");
      return page.locator("#area").inputValue();
    },
  ],
  [
    "check",
    async (page) => {
      await page.locator("#radio").check();
      return page.locator("#radio").isChecked();
    },
  ],
  [
    "check already checked",
    async (page) => {
      await page.locator("#check").check();
      return page.evaluate(() => window.events);
    },
  ],
  [
    "uncheck",
    async (page) => {
      await page.locator("#check").uncheck();
      return page.locator("#check").isChecked();
    },
  ],
  [
    "check not a checkbox",
    (page) => page.locator("#text").check({ timeout: 1000 }),
  ],
  [
    "selectOption by value",
    async (page) => [
      await page.locator("#single").selectOption("b"),
      await page.evaluate(() => window.events),
    ],
  ],
  [
    "selectOption by label",
    (page) => page.locator("#single").selectOption("Beta"),
  ],
  [
    "selectOption by object",
    (page) => page.locator("#single").selectOption({ label: "Beta" }),
  ],
  [
    "selectOption by index",
    (page) => page.locator("#single").selectOption({ index: 1 }),
  ],
  [
    "selectOption multiple",
    (page) => page.locator("#multiple").selectOption(["1", "Three"]),
  ],
  [
    "selectOption missing option",
    (page) => page.locator("#single").selectOption("z", { timeout: 1000 }),
  ],
  [
    "selectOption not a select",
    (page) => page.locator("#text").selectOption("a", { timeout: 1000 }),
  ],
  ["count", (page) => page.locator("input").count()],
  [
    "waitFor attached",
    async (page) => {
      await page.evaluate(() => window.later());
      await page.locator("#added").waitFor({ state: "attached" });
      return page.locator("#added").textContent();
    },
  ],
  [
    "waitFor detached",
    async (page) => {
      await page.evaluate(() => window.later());
      await page.locator("#removed").waitFor({ state: "detached" });
      return page.locator("#removed").count();
    },
  ],
  [
    "waitFor visible",
    async (page) => {
      await page.evaluate(() => window.later());
      await page.locator("#added").waitFor();
      return page.locator("#added").isVisible();
    },
  ],
  [
    "waitFor hidden",
    (page) => page.getByText("Save").nth(2).waitFor({ state: "hidden" }),
  ],
  [
    "waitFor timeout",
    (page) => page.locator("table").waitFor({ timeout: 1000 }),
  ],
  [
    "goto response",
    async (page, _, origin) => {
      const response = await page.goto(`${origin}/other`);
      return [
        response?.status(),
        response?.statusText(),
        response?.ok(),
        response?.url(),
        response?.headers()["content-type"],
        await response?.text(),
        page.url(),
        await page.title(),
      ];
    },
  ],
  [
    "goto json",
    async (page, _, origin) => (await page.goto(`${origin}/data`))?.json(),
  ],
  [
    "goto not found",
    async (page, _, origin) => {
      const response = await page.goto(`${origin}/missing`);
      return [response?.status(), response?.ok()];
    },
  ],
  [
    "goto same document",
    async (page, _, origin) => [
      await page.goto(`${origin}/#fragment`),
      page.url(),
    ],
  ],
  [
    "goto waitUntil",
    async (page, _, origin) => {
      await page.goto(`${origin}/other`, { waitUntil: "domcontentloaded" });
      await page.goto(`${origin}/`, { waitUntil: "commit" });
      await page.waitForLoadState();
      return page.title();
    },
  ],
  ["goto unreachable", (page) => page.goto("http://127.0.0.1:1/")],
  [
    "reload",
    async (page) => {
      await page.locator("#text").fill("changed");
      const response = await page.reload();
      return [response?.status(), await page.locator("#text").inputValue()];
    },
  ],
  [
    "goBack",
    async (page, _, origin) => {
      await page.goto(`${origin}/other`);
      const response = await page.goBack();
      return [response?.status(), page.url(), await page.title()];
    },
  ],
  [
    "goBack within document",
    async (page) => {
      await page.getByText("Link").click();
      await page.waitForURL(/#target$/);
      return [await page.goBack(), page.url()];
    },
  ],
  [
    "goBack without history",
    async (page) => {
      await page.goBack();
      return [await page.goBack(), page.url()];
    },
  ],
  ["waitForLoadState", (page) => page.waitForLoadState("domcontentloaded")],
  [
    "waitForURL glob",
    async (page) => {
      await page.locator("#other").click();
      await page.waitForURL("**/{other,another}");
      return page.url();
    },
  ],
  [
    "waitForURL function",
    async (page) => {
      await page.getByText("Link").click();
      await page.waitForURL((url) => url.hash === "#target");
      return page.url();
    },
  ],
  [
    "waitForURL timeout",
    (page) => page.waitForURL("**/never", { timeout: 1000 }),
  ],
  ["url", async (page) => page.url()],
  ["title", (page) => page.title()],
  ["waitForTimeout", (page) => page.waitForTimeout(10)],
  ["evaluate function", (page) => page.evaluate(([a, b]) => a + b, [1, 2])],
  ["evaluate string", (page) => page.evaluate("document.title")],
  [
    "evaluate promise",
    (page) =>
      page.evaluate(
        () => new Promise((resolve) => setTimeout(resolve, 10, "done")),
      ),
  ],
  [
    "evaluate exception",
    (page) =>
      page.evaluate(() => {
        throw new Error("thrown");
      }),
  ],
  [
    "dialog accept prompt",
    async (page) => {
      const seen: unknown[] = [];
      page.on("dialog", (dialog: Dialog) => {
        seen.push(dialog.type(), dialog.message(), dialog.defaultValue());
        dialog.accept("answer");
      });
      seen.push(await page.evaluate(() => prompt("Name?", "default")));
      return seen;
    },
  ],
  [
    "dialog dismiss confirm",
    async (page) => {
      const seen: unknown[] = [];
      page.on("dialog", (dialog: Dialog) => {
        seen.push(dialog.type(), dialog.message(), dialog.defaultValue());
        dialog.dismiss();
      });
      seen.push(await page.evaluate(() => confirm("Sure?")));
      return seen;
    },
  ],
  ["dialog without listener", (page) => page.evaluate(() => confirm("Sure?"))],
  [
    "dialog after off",
    async (page) => {
      const listener = (dialog: Dialog) => dialog.accept();
      page.on("dialog", listener);
      page.off("dialog", listener);
      return page.evaluate(() => confirm("Sure?"));
    },
  ],
  [
    "waitForEvent dialog",
    async (page) => {
      const dialogPromise = page.waitForEvent("dialog");
      const resultPromise = page.evaluate(() => prompt("Name?", "default"));
      const dialog = await dialogPromise;
      const details = [dialog.type(), dialog.message(), dialog.defaultValue()];
      await dialog.accept("answer");
      return [...details, await resultPromise];
    },
  ],
  [
    "waitForEvent download",
    async (page, _, origin) => {
      const downloadPromise = page.waitForEvent("download");
      await page.locator("#download").click();
      const download = await downloadPromise;
      return [
        download.url() === `${origin}/download`,
        download.suggestedFilename(),
        download.page() === page,
        await download.failure(),
      ];
    },
  ],
  [
    "waitForEvent download redirect",
    async (page, _, origin) => {
      const downloadPromise = page.waitForEvent("download");
      await page.locator("#download-redirect").click();
      const download = await downloadPromise;
      return [
        download.url() === `${origin}/download`,
        download.suggestedFilename(),
        await download.failure(),
      ];
    },
  ],
  [
    "waitForEvent download predicate",
    async (page) => {
      const downloadPromise = page.waitForEvent(
        "download",
        (download: Download) => download.suggestedFilename() === "other.txt",
      );
      await page.locator("#download").click();
      await page.locator("#download-other").click();
      const download = await downloadPromise;
      return [download.suggestedFilename(), await download.failure()];
    },
  ],
  [
    "waitForEvent download timeout",
    (page) => page.waitForEvent("download", { timeout: 500 }),
  ],
  [
    "on and off download",
    async (page) => {
      const seen: string[] = [];
      const listener = (download: Download) => {
        seen.push(download.suggestedFilename());
      };
      page.on("download", listener);
      const first = page.waitForEvent("download");
      await page.locator("#download").click();
      await (await first).failure();
      page.off("download", listener);
      const second = page.waitForEvent("download");
      await page.locator("#download-other").click();
      await (await second).failure();
      return seen;
    },
  ],
  [
    "toBeVisible",
    (page, assert) =>
      assert(page.getByText("Save", { exact: true })).toBeVisible(),
  ],
  [
    "toBeVisible false",
    (page, assert) =>
      assert(page.getByText("Save").nth(2)).toBeVisible({ visible: false }),
  ],
  [
    "not toBeVisible",
    (page, assert) => assert(page.locator("table")).not.toBeVisible(),
  ],
  [
    "toBeVisible failure",
    (page, assert) =>
      assert(page.locator("table")).toBeVisible({ timeout: 1000 }),
  ],
  [
    "toBeVisible strict mode violation",
    (page, assert) =>
      assert(page.getByText("Save")).toBeVisible({ timeout: 1000 }),
  ],
  [
    "toHaveText submit input",
    (page, assert) =>
      assert(page.locator("input[type=submit]").first()).toHaveText("Go", {
        timeout: 1000,
      }),
  ],
  [
    "toHaveText script",
    (page, assert) =>
      assert(page.locator("#script")).toHaveText("Code", { timeout: 1000 }),
  ],
  [
    "toHaveText SVG style",
    (page, assert) =>
      assert(page.locator("#svg")).toHaveText(".a{}Icon", { timeout: 1000 }),
  ],
  [
    "toHaveText shadow root",
    (page, assert) =>
      assert(page.locator("#shadow")).toHaveText("LabelIcon", {
        timeout: 1000,
      }),
  ],
  [
    "toHaveText regular expression",
    (page, assert) =>
      assert(page.getByTestId("panel")).toHaveText(/^pan/i, { timeout: 1000 }),
  ],
  [
    "toHaveText ignoreCase",
    (page, assert) =>
      assert(page.getByTestId("panel")).toHaveText("PANEL", {
        ignoreCase: true,
        timeout: 1000,
      }),
  ],
  [
    "toHaveText failure",
    (page, assert) =>
      assert(page.getByTestId("panel")).toHaveText("Other", { timeout: 1000 }),
  ],
  [
    "not toHaveText",
    (page, assert) =>
      assert(page.getByTestId("panel")).not.toHaveText("Other", {
        timeout: 1000,
      }),
  ],
  [
    "toHaveValue",
    (page, assert) =>
      assert(page.locator("#text")).toHaveValue("initial", { timeout: 1000 }),
  ],
  [
    "toHaveValue regular expression",
    (page, assert) =>
      assert(page.locator("#single")).toHaveValue(/^a$/, { timeout: 1000 }),
  ],
  [
    "toHaveValue failure",
    (page, assert) =>
      assert(page.locator("#text")).toHaveValue("other", { timeout: 1000 }),
  ],
  [
    "toHaveURL",
    (page, assert, origin) =>
      assert(page).toHaveURL(`${origin}/`, { timeout: 1000 }),
  ],
  [
    "toHaveURL regular expression",
    (page, assert) => assert(page).toHaveURL(/127\.0\.0\.1/, { timeout: 1000 }),
  ],
  [
    "toHaveURL ignoreCase",
    (page, assert, origin) =>
      assert(page).toHaveURL(`${origin.toUpperCase()}/`, {
        ignoreCase: true,
        timeout: 1000,
      }),
  ],
  [
    "toHaveURL failure",
    (page, assert, origin) =>
      assert(page).toHaveURL(`${origin}/other`, { timeout: 1000 }),
  ],
];

// Error messages differ between the two, so only the kind of failure is compared
const outcome = async (run: () => Promise<unknown>) => {
  try {
    return { value: await run() };
  } catch (error) {
    const message = (error as Error).message;
    return {
      error: message.includes("strict mode violation")
        ? "strict mode violation"
        : /Timeout:?\s+\d+ms|Timed out/.test(message)
          ? "timeout"
          : "error",
    };
  }
};

const sessions = new Map();
const eventListeners: ((
  source: { tabId: number },
  method: string,
  parameters: object,
) => void)[] = [];
Object.assign(globalThis, {
  chrome: {
    debugger: {
      onEvent: { addListener: (listener) => eventListeners.push(listener) },
      onDetach: { addListener: () => {} },
      sendCommand: (
        target: { tabId: number },
        method: string,
        parameters: object,
      ) => sessions.get(target.tabId).send(method, parameters),
    },
  },
});

let server: Server;
let origin: string;

test.beforeAll(async () => {
  server = createServer((request, response) => {
    const pathname = new URL(request.url ?? "/", "http://localhost").pathname;
    if (pathname === "/download") {
      response
        .writeHead(200, {
          "content-type": "text/plain",
          "content-disposition": 'attachment; filename="sample.txt"',
        })
        .end("downloaded");
      return;
    }
    if (pathname === "/download-other") {
      response
        .writeHead(200, {
          "content-type": "text/plain",
          "content-disposition": 'attachment; filename="other.txt"',
        })
        .end("other");
      return;
    }
    if (pathname === "/download-redirect") {
      response.writeHead(302, { location: "/download" }).end();
      return;
    }
    const page = PAGES[pathname];
    if (page === undefined) {
      response.writeHead(404).end();
      return;
    }
    response.writeHead(200, { "content-type": page[0] }).end(page[1]);
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

test.afterAll(() => {
  server.close();
});

test.describe("Compatibility - Playwright", () => {
  for (const [name, run] of CASES) {
    test(`should match Playwright for ${name}`, async ({ context, page }) => {
      const { Page, expect: assert } = await import("../../lib/page.js");
      const tab = await context.newPage();
      // Keeps Playwright from dismissing the dialogs page.js handles on this tab
      tab.on("dialog", () => {});
      const tabId = sessions.size + 1;
      const session = await context.newCDPSession(tab);
      sessions.set(tabId, session);
      for (const method of [
        "Page.frameNavigated",
        "Page.navigatedWithinDocument",
        "Network.responseReceived",
        "Page.javascriptDialogOpening",
        "Page.downloadWillBegin",
        "Page.downloadProgress",
      ]) {
        session.on(method, (parameters) => {
          for (const listener of eventListeners) {
            listener({ tabId }, method, parameters);
          }
        });
      }
      await session.send("Page.enable");
      await session.send("Network.enable");
      const ours = new Page(
        tabId,
        Date.now() + (test.info().timeout || Number.POSITIVE_INFINITY),
      );
      await page.goto(`${origin}/`);
      await tab.goto(`${origin}/`);

      const expected = await outcome(() => run(page, expect, origin));
      const actual = await outcome(() =>
        run(
          ours as unknown as PlaywrightPage,
          assert as unknown as Expect,
          origin,
        ),
      );
      expect(actual).toStrictEqual(expected);
    });
  }
});
