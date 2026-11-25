import createGitHubRepository from "./automation/create_github_repository.js";
import downloadGitHubFile from "./automation/download_github_file.js";
import { Page } from "./lib/page.js";

const TEMPLATES = [createGitHubRepository, downloadGitHubFile];

const templateSelect = document.getElementById("template");
const csvInput = document.getElementById("csv");
const columnsParagraph = document.getElementById("columns");
const settingsSection = document.getElementById("settings");
const countSpan = document.getElementById("count");
const concurrencyInput = document.getElementById("concurrency");
const timeoutInput = document.getElementById("timeout");
const runButton = document.getElementById("run");
const downloadButton = document.getElementById("download");
const copyButton = document.getElementById("copy");
const cancelButton = document.getElementById("cancel");
const rowsTable = document.getElementById("rows");

let fileName = "";
let cancelled = false;
let cancel = () => {};
let header = [];
let records = [];
let results = [];
let resultColumns = [];

const renderRows = () => {
  rowsTable.replaceChildren();

  const headRow = rowsTable.createTHead().insertRow();
  for (const name of [...header, ...resultColumns, "status"]) {
    const cell = document.createElement("th");
    cell.textContent = name;
    headRow.appendChild(cell);
  }

  const body = rowsTable.createTBody();
  records.forEach((record, index) => {
    const row = body.insertRow();
    for (const name of header) {
      row.insertCell().textContent = record[name];
    }
    const result = results[index];
    for (const name of resultColumns) {
      row.insertCell().textContent = result.values[name] ?? "";
    }
    const status = row.insertCell();
    status.className = result.status;
    status.textContent = result.error || result.status;
  });
};

const renderColumns = () => {
  const template = TEMPLATES[templateSelect.value];
  const missing = template.columns.filter((name) => !header.includes(name));
  resultColumns = template.resultColumns;
  const failed = fileName !== "" && missing.length > 0;
  columnsParagraph.className = failed ? "failed" : "";
  columnsParagraph.textContent = failed
    ? `CSV header must include: ${missing.join(", ")} (found: ${header.map((name) => JSON.stringify(name)).join(", ")})`
    : `Columns: ${[...template.columns, ...template.optionalColumns].join(", ")}`;
  runButton.disabled = missing.length > 0;
  renderRows();
};

const formatResultCsv = () => {
  const table = [
    [...header, ...resultColumns, "error"],
    ...records.map((record, index) => [
      ...header.map((name) => record[name]),
      ...resultColumns.map((name) => results[index].values[name] ?? ""),
      results[index].error,
    ]),
  ];
  // https://www.rfc-editor.org/rfc/rfc4180
  return table
    .map((cells) =>
      cells
        .map((cell) =>
          /[",\r\n]/.test(cell) ? `"${cell.replaceAll('"', '""')}"` : cell,
        )
        .join(","),
    )
    .join("\r\n");
};

csvInput.addEventListener("change", async () => {
  const file = csvInput.files[0];
  if (file === undefined) {
    return;
  }

  // https://www.rfc-editor.org/rfc/rfc4180
  const text = (await file.text()).replace(/^﻿/, "");
  const table = [];
  let cells = [];
  let field = "";
  let quoted = false;
  for (let index = 0; index < text.length; index++) {
    const character = text[index];
    if (quoted) {
      if (character !== '"') {
        field += character;
      } else if (text[index + 1] === '"') {
        field += '"';
        index++;
      } else {
        quoted = false;
      }
    } else if (character === '"') {
      quoted = true;
    } else if (character === ",") {
      cells.push(field);
      field = "";
    } else if (character === "\r" || character === "\n") {
      if (character === "\r" && text[index + 1] === "\n") {
        index++;
      }
      cells.push(field);
      table.push(cells);
      cells = [];
      field = "";
    } else {
      field += character;
    }
  }
  if (field.length > 0 || cells.length > 0) {
    cells.push(field);
    table.push(cells);
  }

  fileName = file.name;
  header = table[0] ?? [];
  records = table
    .slice(1)
    .map((row) =>
      Object.fromEntries(header.map((name, index) => [name, row[index] ?? ""])),
    );
  results = records.map(() => ({ status: "pending", values: {}, error: "" }));
  countSpan.textContent = String(records.length);
  settingsSection.hidden = false;
  downloadButton.disabled = true;
  copyButton.disabled = true;
  renderColumns();
});

const selectTemplate = () => {
  const template = TEMPLATES[templateSelect.value];
  timeoutInput.value =
    template.timeout === undefined ? "" : String(template.timeout);
  renderColumns();
};

templateSelect.addEventListener("change", selectTemplate);

runButton.addEventListener("click", async () => {
  const template = TEMPLATES[templateSelect.value];
  const concurrency = Math.max(1, Number(concurrencyInput.value));
  const timeout =
    timeoutInput.value.trim() === ""
      ? Number.POSITIVE_INFINITY
      : Math.max(1, Number(timeoutInput.value));

  results = records.map(() => ({ status: "pending", values: {}, error: "" }));
  renderColumns();
  runButton.disabled = true;
  csvInput.disabled = true;
  templateSelect.disabled = true;
  downloadButton.disabled = true;
  copyButton.disabled = true;
  cancelled = false;
  const cancellation = new Promise((_, reject) => {
    cancel = () => reject(new Error("cancelled"));
  });
  cancellation.catch(() => {});
  cancelButton.disabled = false;

  const port = chrome.runtime.connect({ name: "automation" });
  let connected = true;
  let automationError;
  const disconnected = new Promise((resolve) => {
    port.onDisconnect.addListener(() => {
      connected = false;
      automationError ??= "Automation connection closed";
      cancelled = true;
      cancel();
      resolve(automationError);
    });
  });
  const waitForReply = (type) =>
    Promise.race([
      disconnected,
      new Promise((resolve) => {
        const listener = (message) => {
          if (message.type === type) {
            port.onMessage.removeListener(listener);
            resolve(message.error);
          }
        };
        port.onMessage.addListener(listener);
      }),
    ]);
  try {
    automationError = await waitForReply("automation:allowed");
    if (automationError !== undefined) {
      return;
    }

    const withPage = async (focused, timeout, callback) => {
      let windowId = null;
      try {
        const window = await chrome.windows.create({
          url: "about:blank",
          focused,
        });
        windowId = window.id;
        if (cancelled) {
          throw new Error("cancelled");
        }
        const tabId = window.tabs[0].id;
        await chrome.debugger.attach({ tabId }, "1.3");
        await chrome.debugger.sendCommand({ tabId }, "Page.enable", {});
        await chrome.debugger.sendCommand({ tabId }, "Network.enable", {});
        return await Promise.race([
          callback(new Page(tabId, Date.now() + timeout)),
          cancellation,
        ]);
      } finally {
        if (windowId !== null) {
          // The user may have closed the window during the run
          await chrome.windows.remove(windowId).catch(() => {});
        }
      }
    };

    // Hooks may wait for the user to sign in
    const runHook = async (name, ...parameters) => {
      if (template[name] === undefined || cancelled) {
        return true;
      }
      try {
        await withPage(true, Number.POSITIVE_INFINITY, (page) =>
          template[name](page, ...parameters),
        );
        return true;
      } catch (error) {
        columnsParagraph.className = "failed";
        columnsParagraph.textContent = `${name} failed: ${cancelled ? "cancelled" : error.message}`;
        return false;
      }
    };

    if (await runHook("beforeAll")) {
      let next = 0;
      await Promise.all(
        Array.from(
          { length: Math.min(concurrency, records.length) },
          async () => {
            while (!cancelled && next < records.length) {
              const index = next++;
              const record = records[index];
              const result = results[index];
              result.status = "running";
              renderRows();

              try {
                result.values =
                  (await withPage(false, timeout, (page) =>
                    template.run(page, record),
                  )) ?? {};
                result.status = "done";
              } catch (error) {
                result.status = "failed";
                result.error = cancelled ? "cancelled" : error.message;
              }
              renderRows();
            }
          },
        ),
      );
    }
    await runHook("afterAll", {
      fileName,
      records,
      results,
      csv: formatResultCsv(),
    });
  } finally {
    if (connected) {
      const restored = waitForReply("automation:restored");
      port.postMessage({ type: "automation:finish" });
      const restoreError = await restored;
      automationError ??= restoreError;
      if (connected) {
        port.disconnect();
      }
    }
    if (automationError !== undefined) {
      columnsParagraph.className = "failed";
      columnsParagraph.textContent = automationError;
    }
    runButton.disabled = false;
    csvInput.disabled = false;
    templateSelect.disabled = false;
    downloadButton.disabled = false;
    copyButton.disabled = false;
    cancelButton.disabled = true;
  }
});

cancelButton.addEventListener("click", () => {
  cancelled = true;
  cancelButton.disabled = true;
  cancel();
});

downloadButton.addEventListener("click", () => {
  const anchor = document.createElement("a");
  anchor.href = URL.createObjectURL(
    new Blob([`﻿${formatResultCsv()}`], { type: "text/csv" }),
  );
  anchor.download = fileName.replace(/\.csv$/i, "-result.csv");
  anchor.click();
  URL.revokeObjectURL(anchor.href);
});

copyButton.addEventListener("click", () => {
  navigator.clipboard.writeText(formatResultCsv());
});

TEMPLATES.forEach((template, index) => {
  templateSelect.add(new Option(template.name, index));
});
selectTemplate();
