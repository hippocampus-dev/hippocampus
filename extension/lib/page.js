// https://playwright.dev/docs/test-timeouts
const EXPECT_TIMEOUT = 5000;
const POLL_INTERVAL = 100;
const NAVIGATION_MARKER = "__hippocampusNavigation";

// https://developer.mozilla.org/en-US/docs/Web/API/UI_Events/Keyboard_event_code_values
const KEYS = {
  Backspace: { code: "Backspace", keyCode: 8 },
  Tab: { code: "Tab", keyCode: 9 },
  Enter: { code: "Enter", keyCode: 13, text: "\r" },
  Shift: { code: "ShiftLeft", keyCode: 16 },
  ShiftLeft: { code: "ShiftLeft", keyCode: 16 },
  Control: { code: "ControlLeft", keyCode: 17 },
  Alt: { code: "AltLeft", keyCode: 18 },
  Escape: { code: "Escape", keyCode: 27 },
  PageUp: { code: "PageUp", keyCode: 33 },
  PageDown: { code: "PageDown", keyCode: 34 },
  End: { code: "End", keyCode: 35 },
  Home: { code: "Home", keyCode: 36 },
  ArrowLeft: { code: "ArrowLeft", keyCode: 37 },
  ArrowUp: { code: "ArrowUp", keyCode: 38 },
  ArrowRight: { code: "ArrowRight", keyCode: 39 },
  ArrowDown: { code: "ArrowDown", keyCode: 40 },
  Insert: { code: "Insert", keyCode: 45 },
  Delete: { code: "Delete", keyCode: 46 },
  Meta: { code: "MetaLeft", keyCode: 91 },
};

const CHARACTER_CODES = {
  " ": { code: "Space", keyCode: 32 },
  ";": { code: "Semicolon", keyCode: 186 },
  "=": { code: "Equal", keyCode: 187 },
  ",": { code: "Comma", keyCode: 188 },
  "-": { code: "Minus", keyCode: 189 },
  ".": { code: "Period", keyCode: 190 },
  "/": { code: "Slash", keyCode: 191 },
  "`": { code: "Backquote", keyCode: 192 },
  "[": { code: "BracketLeft", keyCode: 219 },
  "\\": { code: "Backslash", keyCode: 220 },
  "]": { code: "BracketRight", keyCode: 221 },
  "'": { code: "Quote", keyCode: 222 },
};

// https://github.com/microsoft/playwright/blob/main/packages/playwright-core/src/server/usKeyboardLayout.ts
const SHIFTED_CHARACTERS = {
  1: "!",
  2: "@",
  3: "#",
  4: "$",
  5: "%",
  6: "^",
  7: "&",
  8: "*",
  9: "(",
  0: ")",
  ";": ":",
  "=": "+",
  ",": "<",
  "-": "_",
  ".": ">",
  "/": "?",
  "`": "~",
  "[": "{",
  "\\": "|",
  "]": "}",
  "'": '"',
};

// https://chromedevtools.github.io/devtools-protocol/tot/Input/#method-dispatchKeyEvent
const MODIFIERS = { Alt: 1, Control: 2, Meta: 4, Shift: 8, ShiftLeft: 8 };

const PAGES = new Map();
const PENDING_DOWNLOADS = [];
const DOWNLOADS = new Map();

chrome.debugger.onEvent.addListener((source, method, parameters) => {
  PAGES.get(source.tabId)?.handleEvent(method, parameters);
});

chrome.debugger.onDetach.addListener((source) => {
  const page = PAGES.get(source.tabId);
  if (page !== undefined) {
    PAGES.delete(source.tabId);
    page.detach();
  }
});

// DownloadManagerImpl::CreateNewDownloadItemToStart emits Page.downloadWillBegin and then onCreated: https://source.chromium.org/chromium/chromium/src/+/main:content/browser/download/download_manager_impl.cc
// onDeterminingFilename comes later from DownloadTargetDeterminer::DoNotifyExtensions: https://source.chromium.org/chromium/chromium/src/+/main:chrome/browser/download/download_target_determiner.cc
chrome.downloads?.onCreated?.addListener((item) => {
  const url = item.finalUrl || item.url;
  const index = PENDING_DOWNLOADS.findIndex(
    (download) => download.url() === url,
  );
  if (index === -1) {
    return;
  }
  const [download] = PENDING_DOWNLOADS.splice(index, 1);
  DOWNLOADS.set(item.id, download);
});

// https://developer.chrome.com/docs/extensions/reference/api/downloads#event-onDeterminingFilename
chrome.downloads?.onDeterminingFilename?.addListener((item, suggest) => {
  const download = DOWNLOADS.get(item.id);
  if (download === undefined) {
    suggest();
    return;
  }
  return download.attach(item, suggest);
});

chrome.downloads?.onChanged?.addListener((delta) => {
  const download = DOWNLOADS.get(delta.id);
  if (download === undefined) {
    return;
  }
  if (download.handleChange(delta)) {
    DOWNLOADS.delete(delta.id);
  }
});

const sleep = (milliseconds) =>
  new Promise((resolve) => setTimeout(resolve, milliseconds));

const encodeText = (value) =>
  value instanceof RegExp
    ? { source: value.source, flags: value.flags }
    : { string: String(value) };

const checkOptions = (options, supported, method) => {
  const unsupported = Object.keys(options).filter(
    (name) => !supported.includes(name),
  );
  if (unsupported.length > 0) {
    throw new Error(
      `${method} option ${unsupported.join(", ")} is not implemented`,
    );
  }
};

const strictModeViolation = (chain, count) => ({
  failure: `strict mode violation: ${JSON.stringify(chain)} resolved to ${count} elements`,
});

// https://playwright.dev/docs/network#glob-url-patterns
const globToRegExp = (glob) => {
  let source = "";
  let inBraces = false;
  for (let index = 0; index < glob.length; index++) {
    const character = glob[index];
    if (character === "\\" && index + 1 < glob.length) {
      index++;
      source += glob[index].replace(/[.*+?^${}()|[\]\\/]/g, "\\$&");
    } else if (character === "*") {
      if (glob[index + 1] === "*") {
        index++;
        source += ".*";
      } else {
        source += "[^/]*";
      }
    } else if (character === "{") {
      inBraces = true;
      source += "(";
    } else if (character === "}" && inBraces) {
      inBraces = false;
      source += ")";
    } else if (character === "," && inBraces) {
      source += "|";
    } else {
      source += character.replace(/[.*+?^${}()|[\]\\/]/g, "\\$&");
    }
  }
  return new RegExp(`^${source}$`);
};

const urlMatches = (url, expected, ignoreCase) => {
  if (typeof expected === "function") {
    return expected(new URL(url));
  }
  const pattern =
    expected instanceof RegExp
      ? new RegExp(
          expected.source,
          ignoreCase && !expected.flags.includes("i")
            ? `${expected.flags}i`
            : expected.flags,
        )
      : null;
  if (pattern !== null) {
    return pattern.test(url);
  }
  return ignoreCase
    ? url.toLowerCase() === expected.toLowerCase()
    : url === expected;
};

const keyDefinition = (key) => {
  if (KEYS[key] !== undefined) {
    return { key, ...KEYS[key] };
  }
  const functionKey = /^F([1-9]|1[0-2])$/.exec(key);
  if (functionKey !== null) {
    return { key, code: key, keyCode: 111 + Number(functionKey[1]) };
  }
  const named = /^(?:Key([A-Z])|Digit([0-9]))$/.exec(key);
  if (named !== null) {
    const character = (named[1] ?? named[2]).toLowerCase();
    return {
      key: character,
      code: key,
      keyCode: character.toUpperCase().charCodeAt(0),
      text: character,
      shifted: named[1] ?? SHIFTED_CHARACTERS[named[2]],
    };
  }
  const coded = Object.entries(CHARACTER_CODES).find(
    ([, definition]) => definition.code === key,
  );
  if (coded !== undefined) {
    return {
      key: coded[0],
      ...coded[1],
      text: coded[0],
      shifted: SHIFTED_CHARACTERS[coded[0]],
    };
  }
  if ([...key].length !== 1) {
    throw new Error(`Unknown key: ${key}`);
  }
  const upper = key.toUpperCase();
  if (/[A-Z0-9]/.test(upper)) {
    return {
      key,
      code: /[A-Z]/.test(upper) ? `Key${upper}` : `Digit${key}`,
      keyCode: upper.charCodeAt(0),
      text: key,
    };
  }
  return {
    key,
    ...(CHARACTER_CODES[key] ?? { code: "", keyCode: 0 }),
    text: key,
  };
};

// Runs inside the page, so it must not reference anything outside itself
const query = (chain, operation, argument) => {
  const normalize = (text) => text.replace(/\s+/g, " ").trim();
  const matches = (actual, expected, exact) => {
    if (expected.source !== undefined) {
      return new RegExp(expected.source, expected.flags).test(actual);
    }
    return exact
      ? normalize(actual) === normalize(expected.string)
      : normalize(actual)
          .toLowerCase()
          .includes(normalize(expected.string).toLowerCase());
  };
  const texts = new Map();
  const textOf = (node) => {
    if (node.nodeType === Node.TEXT_NODE) {
      return node.data;
    }
    if (["NOSCRIPT", "SCRIPT", "STYLE", "TEMPLATE"].includes(node.nodeName)) {
      return "";
    }
    if (
      node.localName === "input" &&
      ["button", "reset", "submit"].includes(node.type)
    ) {
      return node.value;
    }
    if (!texts.has(node)) {
      const children = [
        ...node.childNodes,
        ...(node.shadowRoot?.childNodes ?? []),
      ];
      texts.set(node, children.map(textOf).join(""));
    }
    return texts.get(node);
  };
  const isVisible = (element) => {
    const rectangle = element.getBoundingClientRect();
    return (
      rectangle.width > 0 &&
      rectangle.height > 0 &&
      getComputedStyle(element).visibility !== "hidden"
    );
  };
  const isEnabled = (element) =>
    !element.matches(":disabled") &&
    element.closest("[aria-disabled=true]") === null;
  const checkedState = (element) => {
    if (
      element.localName === "input" &&
      ["checkbox", "radio"].includes(element.type)
    ) {
      return element.checked;
    }
    const checked = element.getAttribute("aria-checked");
    return checked === null ? "error" : checked.toLowerCase() === "true";
  };
  const labelTexts = (element) =>
    [
      element.getAttribute("aria-label"),
      ...(element.getAttribute("aria-labelledby") ?? "")
        .split(/\s+/)
        .filter((id) => id !== "")
        .map((id) => document.getElementById(id))
        .filter((label) => label !== null)
        .map(textOf),
      // https://developer.mozilla.org/en-US/docs/Web/API/HTMLInputElement/labels
      ...[...(element.labels ?? [])].map(textOf),
    ].filter((text) => text !== null);
  const scopes = (root) => [
    root,
    ...(root.shadowRoot ? scopes(root.shadowRoot) : []),
    ...[...root.querySelectorAll("*")]
      .filter((element) => element.shadowRoot !== null)
      .flatMap((element) => scopes(element.shadowRoot)),
  ];
  const descendants = (parents) => [
    ...new Set(
      parents.flatMap((parent) =>
        scopes(parent).flatMap((scope) => [...scope.querySelectorAll("*")]),
      ),
    ),
  ];
  const resolve = (chain, roots) => {
    let elements = roots;
    for (const step of chain) {
      switch (step.type) {
        case "css":
          elements = [
            ...new Set(
              elements.flatMap((parent) =>
                scopes(parent).flatMap((scope) => [
                  ...scope.querySelectorAll(step.selector),
                ]),
              ),
            ),
          ];
          break;
        case "xpath":
          elements = [
            ...new Set(
              elements.flatMap((parent) => {
                const snapshot = document.evaluate(
                  step.selector,
                  parent,
                  null,
                  XPathResult.ORDERED_NODE_SNAPSHOT_TYPE,
                  null,
                );
                return Array.from(
                  { length: snapshot.snapshotLength },
                  (_, index) => snapshot.snapshotItem(index),
                ).filter((node) => node.nodeType === Node.ELEMENT_NODE);
              }),
            ),
          ];
          break;
        case "text": {
          const candidates = descendants(elements).filter(
            (element) =>
              !["head", "script", "style", "template"].includes(
                element.localName,
              ) && matches(textOf(element), step.text, step.exact),
          );
          elements = candidates.filter(
            (element) =>
              ![
                ...element.children,
                ...(element.shadowRoot?.children ?? []),
              ].some((child) => candidates.includes(child)),
          );
          break;
        }
        case "label":
          elements = descendants(elements).filter((element) =>
            labelTexts(element).some((text) =>
              matches(text, step.text, step.exact),
            ),
          );
          break;
        case "placeholder":
          elements = descendants(elements).filter(
            (element) =>
              element.hasAttribute("placeholder") &&
              matches(
                element.getAttribute("placeholder"),
                step.text,
                step.exact,
              ),
          );
          break;
        case "testid":
          elements = descendants(elements).filter(
            (element) =>
              element.hasAttribute("data-testid") &&
              matches(element.getAttribute("data-testid"), step.id, true),
          );
          break;
        case "nth": {
          const element = elements.at(step.index);
          elements = element === undefined ? [] : [element];
          break;
        }
        case "filter":
          elements = elements.filter(
            (element) =>
              (step.hasText === undefined ||
                matches(textOf(element), step.hasText, false)) &&
              (step.hasNotText === undefined ||
                !matches(textOf(element), step.hasNotText, false)) &&
              (step.has === undefined ||
                resolve(step.has, [element]).length > 0) &&
              (step.hasNot === undefined ||
                resolve(step.hasNot, [element]).length === 0) &&
              (step.visible === undefined ||
                isVisible(element) === step.visible),
          );
          break;
      }
    }
    return elements;
  };

  let elements;
  try {
    elements = resolve(chain, [document]);
  } catch (error) {
    // A malformed selector fails at once instead of being retried until the timeout
    return { failure: error.message };
  }
  if (operation === "count") {
    return { count: elements.length };
  }
  if (operation === "visible") {
    return {
      count: elements.length,
      visible: elements.length === 1 && isVisible(elements[0]),
    };
  }
  if (elements.length !== 1) {
    return { count: elements.length };
  }
  const element = elements[0];
  switch (operation) {
    case "inspect": {
      if (argument === true) {
        element.scrollIntoView({ block: "center", inline: "center" });
      }
      const rectangle = element.getBoundingClientRect();
      const x = rectangle.x + rectangle.width / 2;
      const y = rectangle.y + rectangle.height / 2;
      const hit = element.getRootNode().elementFromPoint?.(x, y) ?? null;
      return {
        count: 1,
        box: [rectangle.x, rectangle.y, rectangle.width, rectangle.height],
        visible: isVisible(element),
        enabled: isEnabled(element),
        readonly: element.isContentEditable
          ? false
          : ["input", "textarea", "select"].includes(element.localName)
            ? element.hasAttribute("readonly")
            : element.hasAttribute("aria-readonly")
              ? element.getAttribute("aria-readonly").toLowerCase() === "true"
              : "error",
        // https://playwright.dev/docs/actionability#not-inert
        inert: element.closest("[inert]") !== null,
        hit: hit !== null && (hit === element || element.contains(hit)),
        checked: checkedState(element),
      };
    }
    case "focus":
      element.focus();
      if (argument === true) {
        if (["input", "textarea"].includes(element.localName)) {
          element.select();
        } else if (element.isContentEditable) {
          const range = document.createRange();
          range.selectNodeContents(element);
          getSelection().removeAllRanges();
          getSelection().addRange(range);
        }
      }
      return { count: 1 };
    case "select": {
      if (element.localName !== "select") {
        return { count: 1, error: "Element is not a <select> element" };
      }
      const options = argument.map((wanted) =>
        [...element.options].find((option, index) =>
          wanted.valueOrLabel !== undefined
            ? option.value === wanted.valueOrLabel ||
              option.label === wanted.valueOrLabel
            : (wanted.value === undefined || option.value === wanted.value) &&
              (wanted.label === undefined || option.label === wanted.label) &&
              (wanted.index === undefined || index === wanted.index),
        ),
      );
      if (options.includes(undefined)) {
        return { count: 1, pending: true };
      }
      for (const option of element.options) {
        option.selected = options.includes(option);
      }
      element.dispatchEvent(new Event("input", { bubbles: true }));
      element.dispatchEvent(new Event("change", { bubbles: true }));
      return { count: 1, value: options.map((option) => option.value) };
    }
    case "read":
      switch (argument.property) {
        case "textContent":
          return { count: 1, value: element.textContent };
        case "text":
          return { count: 1, value: textOf(element) };
        case "innerText":
          return { count: 1, value: element.innerText };
        case "inputValue":
          return ["input", "textarea", "select"].includes(element.localName)
            ? { count: 1, value: element.value }
            : {
                count: 1,
                error: "Not an <input>, <textarea> or <select> element",
              };
        case "attribute":
          return { count: 1, value: element.getAttribute(argument.name) };
        case "checked": {
          const checked = checkedState(element);
          return checked === "error"
            ? { count: 1, error: "Not a checkbox or radio button" }
            : { count: 1, value: checked };
        }
      }
  }
};

export class Locator {
  constructor(page, chain) {
    this.page = page;
    this.chain = chain;
  }

  #with(step) {
    return new Locator(this.page, [...this.chain, step]);
  }

  #describe() {
    return JSON.stringify(this.chain);
  }

  // https://playwright.dev/docs/locators#locate-by-css-or-xpath
  locator(selector) {
    const engine = /^([a-z-]+)=/.exec(selector);
    if (engine !== null) {
      if (!["css", "xpath"].includes(engine[1])) {
        throw new Error(`The ${engine[1]}= selector engine is not implemented`);
      }
      return this.#with({
        type: engine[1],
        selector: selector.slice(engine[0].length),
      });
    }
    return this.#with({
      type:
        selector.startsWith("//") || selector.startsWith("..")
          ? "xpath"
          : "css",
      selector,
    });
  }

  getByRole() {
    throw new Error("getByRole is not implemented");
  }

  getByText(text, options = {}) {
    checkOptions(options, ["exact"], "getByText");
    return this.#with({
      type: "text",
      text: encodeText(text),
      exact: options.exact === true,
    });
  }

  getByLabel(text, options = {}) {
    checkOptions(options, ["exact"], "getByLabel");
    return this.#with({
      type: "label",
      text: encodeText(text),
      exact: options.exact === true,
    });
  }

  getByPlaceholder(text, options = {}) {
    checkOptions(options, ["exact"], "getByPlaceholder");
    return this.#with({
      type: "placeholder",
      text: encodeText(text),
      exact: options.exact === true,
    });
  }

  getByTestId(id) {
    return this.#with({ type: "testid", id: encodeText(id) });
  }

  first() {
    return this.nth(0);
  }

  last() {
    return this.nth(-1);
  }

  nth(index) {
    return this.#with({ type: "nth", index });
  }

  // https://playwright.dev/docs/api/class-locator#locator-filter
  filter(options = {}) {
    checkOptions(
      options,
      ["has", "hasNot", "hasNotText", "hasText", "visible"],
      "filter",
    );
    return this.#with({
      type: "filter",
      hasText:
        options.hasText === undefined ? undefined : encodeText(options.hasText),
      hasNotText:
        options.hasNotText === undefined
          ? undefined
          : encodeText(options.hasNotText),
      has: options.has?.chain,
      hasNot: options.hasNot?.chain,
      visible: options.visible,
    });
  }

  // https://playwright.dev/docs/actionability
  async #waitForActionable(checks, scroll, options) {
    let previous = null;
    return this.page.poll(
      async () => {
        const state = await this.page.query(this.chain, "inspect", scroll);
        if (state.failure !== undefined) {
          return state;
        }
        if (state.count > 1) {
          return strictModeViolation(this.chain, state.count);
        }
        if (state.count === 0) {
          return undefined;
        }
        if (checks.includes("editable") && state.readonly === "error") {
          return {
            failure:
              "Element is not an <input>, <textarea>, <select> or [contenteditable] and does not have a role allowing [aria-readonly]",
          };
        }
        const stable =
          previous !== null &&
          previous.every((value, index) => value === state.box[index]);
        previous = state.box;
        if (
          options.force !== true &&
          ((checks.includes("visible") && !state.visible) ||
            (checks.includes("stable") && !stable) ||
            (checks.includes("enabled") && !state.enabled) ||
            (checks.includes("editable") &&
              (!state.enabled || state.readonly !== false)) ||
            (checks.includes("inert") && state.inert) ||
            (checks.includes("hit") && !state.hit))
        ) {
          return undefined;
        }
        return state;
      },
      `waiting for ${this.#describe()} to be actionable`,
      options.timeout,
    );
  }

  async #single(operation, argument, timeout) {
    const state = await this.page.poll(
      async () => {
        const state = await this.page.query(this.chain, operation, argument);
        if (state.failure !== undefined) {
          return state;
        }
        if (state.count > 1) {
          return strictModeViolation(this.chain, state.count);
        }
        if (state.count === 0 || state.pending === true) {
          return undefined;
        }
        return state.error === undefined ? state : { failure: state.error };
      },
      `waiting for ${this.#describe()}`,
      timeout,
    );
    return state.value;
  }

  async #mouse(defaultClickCount, options) {
    checkOptions(
      options,
      [
        "button",
        "clickCount",
        "delay",
        "force",
        "modifiers",
        "position",
        "timeout",
      ],
      defaultClickCount === 1 ? "click" : "dblclick",
    );
    const state = await this.#waitForActionable(
      ["visible", "stable", "enabled", "inert", "hit"],
      true,
      options,
    );
    const x = state.box[0] + (options.position?.x ?? state.box[2] / 2);
    const y = state.box[1] + (options.position?.y ?? state.box[3] / 2);
    const button = options.button ?? "left";
    const modifiers = (options.modifiers ?? []).reduce(
      (mask, modifier) => mask | MODIFIERS[this.page.resolveModifier(modifier)],
      0,
    );
    await this.page.send("Input.dispatchMouseEvent", {
      type: "mouseMoved",
      x,
      y,
      modifiers,
    });
    const clickCount = options.clickCount ?? defaultClickCount;
    for (let count = 1; count <= clickCount; count++) {
      await this.page.send("Input.dispatchMouseEvent", {
        type: "mousePressed",
        x,
        y,
        modifiers,
        button,
        buttons: { left: 1, right: 2, middle: 4 }[button],
        clickCount: count,
      });
      if (options.delay !== undefined) {
        await sleep(options.delay);
      }
      await this.page.send("Input.dispatchMouseEvent", {
        type: "mouseReleased",
        x,
        y,
        modifiers,
        button,
        buttons: 0,
        clickCount: count,
      });
    }
  }

  async click(options = {}) {
    await this.#mouse(1, options);
  }

  async dblclick(options = {}) {
    await this.#mouse(2, options);
  }

  async hover(options = {}) {
    checkOptions(options, ["force", "position", "timeout"], "hover");
    const state = await this.#waitForActionable(
      ["visible", "stable", "hit"],
      true,
      options,
    );
    await this.page.send("Input.dispatchMouseEvent", {
      type: "mouseMoved",
      x: state.box[0] + (options.position?.x ?? state.box[2] / 2),
      y: state.box[1] + (options.position?.y ?? state.box[3] / 2),
    });
  }

  async fill(value, options = {}) {
    checkOptions(options, ["force", "timeout"], "fill");
    await this.#waitForActionable(
      ["visible", "enabled", "editable", "inert"],
      false,
      options,
    );
    await this.#single("focus", true, options.timeout);
    if (value === "") {
      await this.page.keyboard.press("Delete");
    } else {
      await this.page.send("Input.insertText", { text: value });
    }
  }

  async press(key, options = {}) {
    checkOptions(options, ["delay", "timeout"], "press");
    await this.#single("focus", false, options.timeout);
    await this.page.keyboard.press(key, { delay: options.delay });
  }

  async pressSequentially(text, options = {}) {
    checkOptions(options, ["delay", "timeout"], "pressSequentially");
    await this.#single("focus", false, options.timeout);
    await this.page.keyboard.type(text, { delay: options.delay });
  }

  async #setChecked(checked, options) {
    checkOptions(
      options,
      ["force", "position", "timeout"],
      checked ? "check" : "uncheck",
    );
    const state = await this.#waitForActionable(
      ["visible", "enabled", "inert"],
      false,
      options,
    );
    if (state.checked === "error") {
      throw new Error("Not a checkbox or radio button");
    }
    if (state.checked === checked) {
      return;
    }
    await this.click(options);
    if (
      (await this.#single("read", { property: "checked" }, options.timeout)) !==
      checked
    ) {
      throw new Error("Clicking the checkbox did not change its state");
    }
  }

  async check(options = {}) {
    await this.#setChecked(true, options);
  }

  async uncheck(options = {}) {
    await this.#setChecked(false, options);
  }

  async selectOption(values, options = {}) {
    checkOptions(options, ["force", "timeout"], "selectOption");
    await this.#waitForActionable(
      ["visible", "enabled", "inert"],
      false,
      options,
    );
    return this.#single(
      "select",
      (Array.isArray(values) ? values : [values]).map((value) =>
        typeof value === "string" ? { valueOrLabel: value } : value,
      ),
      options.timeout,
    );
  }

  async setInputFiles() {
    throw new Error("setInputFiles is not implemented");
  }

  async textContent(options = {}) {
    return this.#single("read", { property: "textContent" }, options.timeout);
  }

  async innerText(options = {}) {
    return this.#single("read", { property: "innerText" }, options.timeout);
  }

  async inputValue(options = {}) {
    return this.#single("read", { property: "inputValue" }, options.timeout);
  }

  async getAttribute(name, options = {}) {
    return this.#single(
      "read",
      { property: "attribute", name },
      options.timeout,
    );
  }

  async isChecked(options = {}) {
    return this.#single("read", { property: "checked" }, options.timeout);
  }

  async isVisible() {
    const state = await this.page.poll(
      () => this.page.query(this.chain, "visible"),
      `checking ${this.#describe()}`,
    );
    if (state.count > 1) {
      throw new Error(strictModeViolation(this.chain, state.count).failure);
    }
    return state.visible;
  }

  async count() {
    const state = await this.page.poll(
      () => this.page.query(this.chain, "count"),
      `counting ${this.#describe()}`,
    );
    return state.count;
  }

  async waitFor(options = {}) {
    checkOptions(options, ["state", "timeout"], "waitFor");
    const wanted = options.state ?? "visible";
    await this.page.poll(
      async () => {
        const state = await this.page.query(this.chain, "visible");
        if (state.failure !== undefined) {
          return state;
        }
        if (state.count > 1 && (wanted === "visible" || wanted === "hidden")) {
          return strictModeViolation(this.chain, state.count);
        }
        switch (wanted) {
          case "attached":
            return state.count > 0 ? state : undefined;
          case "detached":
            return state.count === 0 ? state : undefined;
          case "visible":
            return state.visible ? state : undefined;
          case "hidden":
            return state.visible ? undefined : state;
        }
      },
      `waiting for ${this.#describe()} to be ${wanted}`,
      options.timeout,
    );
  }
}

// https://playwright.dev/docs/api/class-response
class Response {
  #page;
  #parameters;

  constructor(page, parameters) {
    this.#page = page;
    this.#parameters = parameters;
  }

  status() {
    return this.#parameters.response.status;
  }

  statusText() {
    return this.#parameters.response.statusText;
  }

  ok() {
    return this.status() >= 200 && this.status() <= 299;
  }

  url() {
    return this.#parameters.response.url;
  }

  headers() {
    return Object.fromEntries(
      Object.entries(this.#parameters.response.headers)
        .map(([name, value]) => [name.toLowerCase(), value])
        .filter(([name]) => name !== "set-cookie"),
    );
  }

  async text() {
    const body = await this.#page.send("Network.getResponseBody", {
      requestId: this.#parameters.requestId,
    });
    return body.base64Encoded
      ? new TextDecoder().decode(
          Uint8Array.from(atob(body.body), (character) =>
            character.charCodeAt(0),
          ),
        )
      : body.body;
  }

  async json() {
    return JSON.parse(await this.text());
  }

  async body() {
    throw new Error("body is not implemented");
  }

  request() {
    throw new Error("request is not implemented");
  }
}

// https://playwright.dev/docs/api/class-dialog
class Dialog {
  #page;
  #parameters;

  constructor(page, parameters) {
    this.#page = page;
    this.#parameters = parameters;
  }

  type() {
    return this.#parameters.type;
  }

  message() {
    return this.#parameters.message;
  }

  defaultValue() {
    return this.#parameters.type === "prompt"
      ? (this.#parameters.defaultPrompt ?? "")
      : "";
  }

  page() {
    return this.#page;
  }

  async accept(promptText) {
    await this.#page.send("Page.handleJavaScriptDialog", {
      accept: true,
      promptText,
    });
  }

  async dismiss() {
    await this.#page.send("Page.handleJavaScriptDialog", { accept: false });
  }
}

// https://playwright.dev/docs/api/class-download
class Download {
  #page;
  #url;
  #suggestedFilename;
  #controlled;
  #id = null;
  #suggest = null;
  #decision = null;
  #path = null;
  #error = null;
  #failure = undefined;
  #resolveCompletion;
  #completion;

  constructor(page, parameters, controlled) {
    this.#page = page;
    this.#url = parameters.url;
    this.#suggestedFilename = parameters.suggestedFilename;
    this.#controlled = controlled;
    this.#completion = new Promise((resolve) => {
      this.#resolveCompletion = resolve;
    });
  }

  url() {
    return this.#url;
  }

  suggestedFilename() {
    return this.#suggestedFilename;
  }

  page() {
    return this.#page;
  }

  #finish(failure) {
    if (this.#failure !== undefined) {
      return;
    }
    this.#failure = failure;
    const index = PENDING_DOWNLOADS.indexOf(this);
    if (index !== -1) {
      PENDING_DOWNLOADS.splice(index, 1);
    }
    this.#resolveCompletion(failure);
  }

  #applyDecision() {
    if (this.#suggest === null || this.#decision === null) {
      return false;
    }
    const suggest = this.#suggest;
    this.#suggest = null;
    if (this.#decision.type === "saveAs") {
      suggest({
        filename: this.#decision.filename,
        conflictAction: "overwrite",
      });
      return false;
    }
    if (this.#decision.type === "cancel" && this.#id !== null) {
      chrome.downloads
        ?.cancel(this.#id)
        .catch(() => {})
        .finally(() => suggest());
      return true;
    }
    suggest();
    return false;
  }

  attach(item, suggest) {
    this.#id = item.id;
    if (this.#failure !== undefined) {
      suggest();
      return false;
    }
    this.#suggest = suggest;
    if (!this.#controlled && this.#decision === null) {
      this.#decision = { type: "default" };
    }
    if (this.#decision !== null) {
      return this.#applyDecision();
    }
    return true;
  }

  handleChange(delta) {
    if (
      delta.filename?.current !== undefined &&
      delta.filename.current !== ""
    ) {
      this.#path = delta.filename.current;
    }
    if (delta.error?.current !== undefined) {
      this.#error = delta.error.current;
    }
    if (delta.state?.current === "complete") {
      this.#finish(null);
      return true;
    }
    if (delta.state?.current === "interrupted") {
      const error = delta.error?.current ?? this.#error;
      this.#finish(
        error === "USER_CANCELED" ? "canceled" : (error ?? "interrupted"),
      );
      return true;
    }
    return false;
  }

  handleProgress(parameters) {
    if (parameters.state === "inProgress") {
      return false;
    }
    if (this.#id === null) {
      this.#finish(parameters.state === "completed" ? null : "canceled");
    }
    return true;
  }

  release() {
    if (this.#decision === null) {
      this.#decision = { type: "default" };
      this.#applyDecision();
    }
  }

  dispose() {
    const index = PENDING_DOWNLOADS.indexOf(this);
    if (index !== -1) {
      PENDING_DOWNLOADS.splice(index, 1);
    }
    this.release();
  }

  async #waitForCompletion() {
    if (this.#failure !== undefined) {
      return this.#failure;
    }
    const remaining = this.#page.remainingTime();
    if (remaining === Number.POSITIVE_INFINITY) {
      return this.#completion;
    }
    if (remaining <= 0) {
      throw new Error("Timed out waiting for the download to finish");
    }
    let timer = null;
    try {
      return await Promise.race([
        this.#completion,
        new Promise((_, reject) => {
          timer = setTimeout(() => {
            reject(new Error("Timed out waiting for the download to finish"));
          }, remaining);
        }),
      ]);
    } finally {
      if (timer !== null) {
        clearTimeout(timer);
      }
    }
  }

  async failure() {
    this.release();
    return this.#waitForCompletion();
  }

  async path() {
    const failure = await this.failure();
    if (failure !== null) {
      throw new Error(`Download failed: ${failure}`);
    }
    if (this.#path === null) {
      throw new Error("Download path is not available");
    }
    return this.#path;
  }

  async saveAs(path) {
    const segments = typeof path === "string" ? path.split(/[/\\]/) : [];
    // https://developer.chrome.com/docs/extensions/reference/api/downloads#type-FilenameSuggestion
    if (
      typeof path !== "string" ||
      path === "" ||
      path.startsWith("/") ||
      path.startsWith("\\") ||
      path.endsWith("/") ||
      path.endsWith("\\") ||
      /^[A-Za-z]:/.test(path) ||
      segments.includes(".") ||
      segments.includes("..")
    ) {
      throw new Error(
        `saveAs path must be a relative path within the Downloads directory: ${path}`,
      );
    }
    if (chrome.downloads === undefined) {
      throw new Error("chrome.downloads is not available");
    }
    if (this.#decision !== null) {
      throw new Error("Download filename has already been determined");
    }
    this.#decision = { type: "saveAs", filename: path.replaceAll("\\", "/") };
    this.#applyDecision();
    const failure = await this.#waitForCompletion();
    if (failure !== null) {
      throw new Error(`Download failed: ${failure}`);
    }
  }

  async cancel() {
    if (this.#failure !== undefined) {
      return;
    }
    if (chrome.downloads === undefined) {
      throw new Error("chrome.downloads is not available");
    }
    if (this.#decision === null) {
      this.#decision = { type: "cancel" };
      this.#applyDecision();
    } else if (this.#id !== null) {
      await chrome.downloads.cancel(this.#id).catch(() => {});
    }
    await this.#waitForCompletion();
  }

  async delete() {
    const failure = await this.failure();
    if (failure !== null) {
      return;
    }
    if (chrome.downloads === undefined || this.#id === null) {
      throw new Error("chrome.downloads is not available");
    }
    await chrome.downloads.removeFile(this.#id);
  }

  async createReadStream() {
    throw new Error("createReadStream is not implemented");
  }
}

export class Page {
  #tabId;
  #deadline;
  #url = "about:blank";
  #mainFrameId = null;
  #loaderId = null;
  #responses = new Map();
  #listeners = { dialog: [], download: [] };
  #downloads = new Map();

  constructor(tabId, deadline) {
    this.#tabId = tabId;
    this.#deadline = deadline;
    PAGES.set(tabId, this);
    this.keyboard = {
      press: async (key, options = {}) => {
        const parts = [];
        let current = "";
        for (const character of key) {
          if (character === "+" && current !== "") {
            parts.push(current);
            current = "";
          } else {
            current += character;
          }
        }
        parts.push(current);
        const definitions = parts.map((part) =>
          keyDefinition(this.resolveModifier(part)),
        );
        const modifiers = definitions.slice(0, -1);
        let mask = 0;
        for (const modifier of modifiers) {
          mask |= MODIFIERS[modifier.key] ?? 0;
          await this.#key("rawKeyDown", modifier, mask);
        }
        let main = definitions.at(-1);
        if ((mask & MODIFIERS.Shift) !== 0 && main.shifted !== undefined) {
          main = { ...main, key: main.shifted, text: main.shifted };
        }
        const printable = (mask & ~MODIFIERS.Shift) === 0;
        await this.#key(
          printable && main.text !== undefined ? "keyDown" : "rawKeyDown",
          printable ? main : { ...main, text: undefined },
          mask,
        );
        if (options.delay !== undefined) {
          await sleep(options.delay);
        }
        await this.#key("keyUp", main, mask);
        for (const modifier of modifiers.reverse()) {
          mask &= ~(MODIFIERS[modifier.key] ?? 0);
          await this.#key("keyUp", modifier, mask);
        }
      },
      type: async (text, options = {}) => {
        for (const character of text) {
          const definition = keyDefinition(character);
          await this.#key("keyDown", definition, 0);
          await this.#key("keyUp", definition, 0);
          if (options.delay !== undefined) {
            await sleep(options.delay);
          }
        }
      },
    };
  }

  // https://playwright.dev/docs/api/class-keyboard#keyboard-press
  resolveModifier(key) {
    if (key !== "ControlOrMeta") {
      return key;
    }
    return navigator.userAgentData?.platform === "macOS" ||
      navigator.platform.startsWith("Mac")
      ? "Meta"
      : "Control";
  }

  handleEvent(method, parameters) {
    switch (method) {
      case "Page.frameNavigated":
        if (parameters.frame.parentId === undefined) {
          this.#mainFrameId = parameters.frame.id;
          this.#loaderId = parameters.frame.loaderId;
          this.#url =
            parameters.frame.url + (parameters.frame.urlFragment ?? "");
        }
        break;
      case "Page.navigatedWithinDocument":
        if (parameters.frameId === this.#mainFrameId) {
          this.#url = parameters.url;
        }
        break;
      case "Network.responseReceived":
        if (parameters.type === "Document") {
          this.#responses.set(parameters.loaderId, parameters);
        }
        break;
      case "Page.javascriptDialogOpening": {
        // https://playwright.dev/docs/dialogs
        if (this.#listeners.dialog.length === 0) {
          this.send("Page.handleJavaScriptDialog", { accept: false });
          break;
        }
        const dialog = new Dialog(this, parameters);
        for (const listener of [...this.#listeners.dialog]) {
          listener(dialog);
        }
        break;
      }
      case "Page.downloadWillBegin": {
        const controlled = this.#listeners.download.length > 0;
        const download = new Download(this, parameters, controlled);
        this.#downloads.set(parameters.guid, download);
        if (chrome.downloads !== undefined) {
          PENDING_DOWNLOADS.push(download);
        }
        for (const listener of [...this.#listeners.download]) {
          listener(download);
        }
        break;
      }
      case "Page.downloadProgress": {
        const download = this.#downloads.get(parameters.guid);
        if (download?.handleProgress(parameters)) {
          this.#downloads.delete(parameters.guid);
        }
        break;
      }
    }
  }

  detach() {
    for (const download of this.#downloads.values()) {
      download.dispose();
    }
    this.#downloads.clear();
  }

  remainingTime() {
    return this.#deadline === Number.POSITIVE_INFINITY
      ? Number.POSITIVE_INFINITY
      : this.#deadline - Date.now();
  }

  on(event, listener) {
    if (this.#listeners[event] === undefined) {
      throw new Error(`The ${event} event is not implemented`);
    }
    this.#listeners[event].push(listener);
    return this;
  }

  off(event, listener) {
    if (this.#listeners[event] === undefined) {
      throw new Error(`The ${event} event is not implemented`);
    }
    this.#listeners[event] = this.#listeners[event].filter(
      (registered) => registered !== listener,
    );
    return this;
  }

  // https://playwright.dev/docs/api/class-page#page-wait-for-event
  async waitForEvent(event, optionsOrPredicate) {
    if (this.#listeners[event] === undefined) {
      throw new Error(`The ${event} event is not implemented`);
    }
    const options =
      typeof optionsOrPredicate === "function"
        ? { predicate: optionsOrPredicate }
        : (optionsOrPredicate ?? {});
    checkOptions(options, ["predicate", "timeout"], "waitForEvent");
    const deadline = Math.min(
      this.#deadline,
      options.timeout === undefined || options.timeout === 0
        ? Number.POSITIVE_INFINITY
        : Date.now() + options.timeout,
    );
    return new Promise((resolve, reject) => {
      let settled = false;
      let timer = null;
      const settle = (callback) => {
        if (settled) {
          return;
        }
        settled = true;
        this.off(event, listener);
        if (timer !== null) {
          clearTimeout(timer);
        }
        callback();
      };
      const releaseIfSoleDownloadListener = (value) => {
        if (
          event === "download" &&
          this.#listeners.download.length === 1 &&
          this.#listeners.download[0] === listener
        ) {
          value.release();
        }
      };
      const listener = async (value) => {
        try {
          if (
            options.predicate !== undefined &&
            !(await options.predicate(value))
          ) {
            releaseIfSoleDownloadListener(value);
            return;
          }
        } catch (error) {
          releaseIfSoleDownloadListener(value);
          settle(() => reject(error));
          return;
        }
        settle(() => resolve(value));
      };
      this.on(event, listener);
      if (deadline !== Number.POSITIVE_INFINITY) {
        timer = setTimeout(
          () => {
            settle(() =>
              reject(new Error(`Timed out waiting for event "${event}"`)),
            );
          },
          Math.max(0, deadline - Date.now()),
        );
      }
    });
  }

  async #key(type, definition, modifiers) {
    await this.send("Input.dispatchKeyEvent", {
      type,
      modifiers,
      key: definition.key,
      code: definition.code,
      windowsVirtualKeyCode: definition.keyCode,
      text: type === "keyDown" ? definition.text : undefined,
    });
  }

  async send(method, parameters) {
    return chrome.debugger.sendCommand(
      { tabId: this.#tabId },
      method,
      parameters,
    );
  }

  async #evaluateExpression(expression) {
    const response = await this.send("Runtime.evaluate", {
      expression,
      awaitPromise: true,
      returnByValue: true,
    });
    if (response.exceptionDetails !== undefined) {
      throw new Error(
        response.exceptionDetails.exception?.description ??
          response.exceptionDetails.text,
      );
    }
    return response.result.value;
  }

  async evaluate(callback, argument) {
    return this.#evaluateExpression(
      typeof callback === "string"
        ? callback
        : `(${callback.toString()})(${JSON.stringify(argument)})`,
    );
  }

  async query(chain, operation, argument) {
    return this.#evaluateExpression(
      `(${query.toString()})(${JSON.stringify(chain)}, ${JSON.stringify(operation)}, ${JSON.stringify(argument)})`,
    );
  }

  // The probe returns undefined to keep waiting, an object with failure to give up, or the result
  async poll(probe, description, timeout) {
    const deadline = Math.min(
      this.#deadline,
      timeout === undefined || timeout === 0
        ? Number.POSITIVE_INFINITY
        : Date.now() + timeout,
    );
    let lastError = null;
    while (Date.now() < deadline) {
      if (PAGES.get(this.#tabId) !== this) {
        throw new Error("Target page, context or browser has been closed");
      }
      let result;
      try {
        result = await probe();
      } catch (error) {
        lastError = error;
      }
      if (result?.failure !== undefined) {
        throw new Error(result.failure);
      }
      if (result !== undefined) {
        return result;
      }
      await sleep(POLL_INTERVAL);
    }
    throw new Error(
      lastError === null
        ? `Timed out ${description}`
        : `Timed out ${description}: ${lastError.message}`,
    );
  }

  locator(selector) {
    return new Locator(this, []).locator(selector);
  }

  getByRole() {
    throw new Error("getByRole is not implemented");
  }

  getByText(text, options) {
    return new Locator(this, []).getByText(text, options);
  }

  getByLabel(text, options) {
    return new Locator(this, []).getByLabel(text, options);
  }

  getByPlaceholder(text, options) {
    return new Locator(this, []).getByPlaceholder(text, options);
  }

  getByTestId(id) {
    return new Locator(this, []).getByTestId(id);
  }

  // The action returns false when the navigation stays in the same document
  async #navigate(action, options) {
    checkOptions(options, ["timeout", "waitUntil"], "navigation");
    const token = crypto.randomUUID();
    await this.evaluate(
      ([name, token]) => {
        window[name] = token;
      },
      [NAVIGATION_MARKER, token],
    ).catch(() => {});
    if ((await action()) === false) {
      return null;
    }
    const waitUntil = options.waitUntil ?? "load";
    if (waitUntil === "networkidle") {
      throw new Error("waitUntil 'networkidle' is not implemented");
    }
    await this.poll(
      () =>
        this.evaluate(
          ([name, token, waitUntil]) =>
            window[name] !== token &&
            (waitUntil === "commit" ||
              document.readyState === "complete" ||
              (waitUntil === "domcontentloaded" &&
                document.readyState !== "loading"))
              ? true
              : undefined,
          [NAVIGATION_MARKER, token, waitUntil],
        ),
      "waiting for the navigation",
      options.timeout,
    );
    return this.#response(this.#loaderId);
  }

  // https://playwright.dev/docs/api/class-page#page-goto
  #response(loaderId) {
    const parameters = this.#responses.get(loaderId);
    return parameters === undefined ? null : new Response(this, parameters);
  }

  async goto(url, options = {}) {
    return this.#navigate(async () => {
      const response = await this.send("Page.navigate", { url });
      if (response.errorText !== undefined) {
        throw new Error(`${url}: ${response.errorText}`);
      }
      // https://chromedevtools.github.io/devtools-protocol/tot/Page/#method-navigate
      if (response.loaderId === undefined) {
        await this.waitForURL((current) => current.href === new URL(url).href, {
          timeout: options.timeout,
          waitUntil: "commit",
        });
        return false;
      }
      return true;
    }, options);
  }

  async reload(options = {}) {
    return this.#navigate(() => this.send("Page.reload", {}), options);
  }

  async goBack(options = {}) {
    checkOptions(options, ["timeout", "waitUntil"], "goBack");
    const history = await this.send("Page.getNavigationHistory", {});
    const entry = history.entries[history.currentIndex - 1];
    // https://playwright.dev/docs/api/class-page#page-go-back
    if (entry === undefined) {
      return null;
    }
    const loaderId = this.#loaderId;
    await this.send("Page.navigateToHistoryEntry", { entryId: entry.id });
    await this.waitForURL((url) => url.href === entry.url, options);
    return this.#loaderId === loaderId ? null : this.#response(this.#loaderId);
  }

  async waitForLoadState(state = "load", options = {}) {
    checkOptions(options, ["timeout"], "waitForLoadState");
    if (state === "networkidle") {
      throw new Error("waitForLoadState('networkidle') is not implemented");
    }
    await this.poll(
      () =>
        this.evaluate(
          (state) =>
            document.readyState === "complete" ||
            (state === "domcontentloaded" && document.readyState !== "loading")
              ? true
              : undefined,
          state,
        ),
      `waiting for the ${state} state`,
      options.timeout,
    );
  }

  // https://playwright.dev/docs/api/class-page#page-wait-for-url
  async waitForURL(url, options = {}) {
    checkOptions(options, ["timeout", "waitUntil"], "waitForURL");
    const expected = typeof url === "string" ? globToRegExp(url) : url;
    await this.poll(
      () => (urlMatches(this.url(), expected, false) ? true : undefined),
      `waiting for the URL ${url}`,
      options.timeout,
    );
    const waitUntil = options.waitUntil ?? "load";
    if (waitUntil !== "commit") {
      await this.waitForLoadState(waitUntil, { timeout: options.timeout });
    }
  }

  url() {
    return this.#url;
  }

  async title() {
    return this.poll(
      () => this.evaluate(() => document.title),
      "reading the title",
    );
  }

  async waitForTimeout(milliseconds) {
    await sleep(milliseconds);
  }

  async screenshot() {
    throw new Error("screenshot is not implemented");
  }
}

// https://playwright.dev/docs/test-assertions
class Expectation {
  #target;
  #negated;

  constructor(target, negated) {
    this.#target = target;
    this.#negated = negated;
  }

  get not() {
    return new Expectation(this.#target, !this.#negated);
  }

  async #assert(probe, description, options) {
    const page =
      this.#target instanceof Page ? this.#target : this.#target.page;
    await page.poll(
      async () => {
        const value = await probe();
        if (value?.failure !== undefined) {
          return value;
        }
        return value !== this.#negated ? true : undefined;
      },
      `expecting ${this.#negated ? "not " : ""}${description}`,
      options.timeout ?? EXPECT_TIMEOUT,
    );
  }

  async #read(property) {
    const state = await this.#target.page.query(this.#target.chain, "read", {
      property,
    });
    if (state.failure !== undefined) {
      return state;
    }
    if (state.count > 1) {
      return strictModeViolation(this.#target.chain, state.count);
    }
    return { value: state.count === 1 ? (state.value ?? null) : null };
  }

  async toBeVisible(options = {}) {
    checkOptions(options, ["timeout", "visible"], "toBeVisible");
    await this.#assert(
      async () => {
        const state = await this.#target.page.query(
          this.#target.chain,
          "visible",
        );
        if (state.failure !== undefined) {
          return state;
        }
        return state.count > 1
          ? strictModeViolation(this.#target.chain, state.count)
          : state.visible === (options.visible ?? true);
      },
      "to be visible",
      options,
    );
  }

  async toHaveText(expected, options = {}) {
    checkOptions(options, ["ignoreCase", "timeout"], "toHaveText");
    const normalize = (text) => {
      const normalized = text.replace(/\s+/g, " ").trim();
      return options.ignoreCase === true
        ? normalized.toLowerCase()
        : normalized;
    };
    await this.#assert(
      async () => {
        const read = await this.#read("text");
        if (read.failure !== undefined) {
          return read;
        }
        if (read.value === null) {
          return false;
        }
        return expected instanceof RegExp
          ? new RegExp(
              expected.source,
              options.ignoreCase === true && !expected.flags.includes("i")
                ? `${expected.flags}i`
                : expected.flags,
            ).test(read.value)
          : normalize(read.value) === normalize(expected);
      },
      `to have text ${expected}`,
      options,
    );
  }

  async toHaveValue(expected, options = {}) {
    checkOptions(options, ["timeout"], "toHaveValue");
    await this.#assert(
      async () => {
        const read = await this.#read("inputValue");
        if (read.failure !== undefined) {
          return read;
        }
        return (
          read.value !== null &&
          (expected instanceof RegExp
            ? expected.test(read.value)
            : read.value === expected)
        );
      },
      `to have value ${expected}`,
      options,
    );
  }

  // https://playwright.dev/docs/api/class-pageassertions#page-assertions-to-have-url
  async toHaveURL(expected, options = {}) {
    checkOptions(options, ["ignoreCase", "timeout"], "toHaveURL");
    await this.#assert(
      () =>
        urlMatches(this.#target.url(), expected, options.ignoreCase === true),
      `to have URL ${expected}`,
      options,
    );
  }
}

export function expect(target) {
  return new Expectation(target, false);
}
