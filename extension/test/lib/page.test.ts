import { beforeEach, describe, expect, test } from "vitest";

type DebuggerEventListener = (
  source: { tabId: number },
  method: string,
  parameters: Record<string, unknown>,
) => void;
type DebuggerDetachListener = (source: { tabId: number }) => void;
type DownloadCreatedListener = (item: {
  id: number;
  url: string;
  finalUrl: string;
}) => void;
type DeterminingFilenameListener = (
  item: {
    id: number;
    url: string;
    finalUrl: string;
    filename: string;
  },
  suggest: (suggestion?: { filename: string; conflictAction?: string }) => void,
) => boolean | undefined;
type DownloadChangedListener = (delta: {
  id: number;
  filename?: { current?: string };
  state?: { current?: string };
  error?: { current?: string };
}) => void;

const debuggerEventListeners: DebuggerEventListener[] = [];
const debuggerDetachListeners: DebuggerDetachListener[] = [];
const downloadCreatedListeners: DownloadCreatedListener[] = [];
const determiningFilenameListeners: DeterminingFilenameListener[] = [];
const downloadChangedListeners: DownloadChangedListener[] = [];

const cancelledIds: number[] = [];
const removedIds: number[] = [];

Object.assign(globalThis, {
  chrome: {
    debugger: {
      onEvent: {
        addListener: (listener: DebuggerEventListener) =>
          debuggerEventListeners.push(listener),
      },
      onDetach: {
        addListener: (listener: DebuggerDetachListener) =>
          debuggerDetachListeners.push(listener),
      },
      sendCommand: async () => ({}),
    },
    downloads: {
      onCreated: {
        addListener: (listener: DownloadCreatedListener) =>
          downloadCreatedListeners.push(listener),
      },
      onDeterminingFilename: {
        addListener: (listener: DeterminingFilenameListener) =>
          determiningFilenameListeners.push(listener),
      },
      onChanged: {
        addListener: (listener: DownloadChangedListener) =>
          downloadChangedListeners.push(listener),
      },
      cancel: async (id: number) => {
        cancelledIds.push(id);
        for (const listener of downloadChangedListeners) {
          listener({
            id,
            state: { current: "interrupted" },
            error: { current: "USER_CANCELED" },
          });
        }
      },
      removeFile: async (id: number) => {
        removedIds.push(id);
      },
    },
  },
});

const emitDebuggerEvent = (
  tabId: number,
  method: string,
  parameters: Record<string, unknown>,
) => {
  for (const listener of debuggerEventListeners) {
    listener({ tabId }, method, parameters);
  }
};

const emitDetach = (tabId: number) => {
  for (const listener of debuggerDetachListeners) {
    listener({ tabId });
  }
};

const createdIds = new Set<number>();

const emitCreated = (item: { id: number; url: string; finalUrl: string }) => {
  createdIds.add(item.id);
  for (const listener of downloadCreatedListeners) {
    listener(item);
  }
};

const emitDetermining = (item: {
  id: number;
  url: string;
  finalUrl: string;
  filename: string;
}) => {
  if (!createdIds.has(item.id)) {
    emitCreated(item);
  }
  let suggestion: { filename: string; conflictAction?: string } | undefined;
  let suggested = false;
  let resolveSuggested!: () => void;
  const suggestedPromise = new Promise<void>((resolve) => {
    resolveSuggested = resolve;
  });
  let isAsync = false;
  for (const listener of determiningFilenameListeners) {
    const result = listener(item, (value) => {
      if (suggested) {
        throw new Error("suggest must only be called once");
      }
      suggested = true;
      suggestion = value;
      resolveSuggested();
    });
    if (result === true) {
      isAsync = true;
    }
  }
  return {
    isAsync,
    suggested: () => suggested,
    suggestion: () => suggestion,
    waitSuggested: () => suggestedPromise,
  };
};

const emitChanged = (delta: {
  id: number;
  filename?: { current?: string };
  state?: { current?: string };
  error?: { current?: string };
}) => {
  for (const listener of downloadChangedListeners) {
    listener(delta);
  }
};

let nextTabId = 100;
let nextDownloadId = 1;

describe("Page", () => {
  beforeEach(() => {
    cancelledIds.length = 0;
    removedIds.length = 0;
  });

  test("saves a download to a relative path within Downloads via saveAs", async () => {
    const { Page } = await import("../../lib/page.js");
    const tabId = nextTabId++;
    const downloadId = nextDownloadId++;
    const page = new Page(tabId, Number.POSITIVE_INFINITY);

    const downloadPromise = page.waitForEvent("download");
    emitDebuggerEvent(tabId, "Page.downloadWillBegin", {
      guid: `guid-${downloadId}`,
      url: "https://example.com/files/report.pdf",
      suggestedFilename: "report.pdf",
    });
    const determining = emitDetermining({
      id: downloadId,
      url: "https://example.com/redirect",
      finalUrl: "https://example.com/files/report.pdf",
      filename: "report.pdf",
    });
    expect(determining.isAsync).toBe(true);
    expect(determining.suggested()).toBe(false);

    const download = await downloadPromise;
    const savePromise = download.saveAs("2026/01/custom.pdf");
    await determining.waitSuggested();
    expect(determining.suggestion()).toStrictEqual({
      filename: "2026/01/custom.pdf",
      conflictAction: "overwrite",
    });

    emitChanged({
      id: downloadId,
      filename: { current: "/home/user/Downloads/2026/01/custom.pdf" },
    });
    emitChanged({ id: downloadId, state: { current: "complete" } });

    await savePromise;
    expect(await download.path()).toBe(
      "/home/user/Downloads/2026/01/custom.pdf",
    );
    expect(await download.failure()).toBeNull();
  });

  test("supports saveAs called before onDeterminingFilename fires", async () => {
    const { Page } = await import("../../lib/page.js");
    const tabId = nextTabId++;
    const downloadId = nextDownloadId++;
    const page = new Page(tabId, Number.POSITIVE_INFINITY);

    const downloadPromise = page.waitForEvent("download");
    emitDebuggerEvent(tabId, "Page.downloadWillBegin", {
      guid: `guid-${downloadId}`,
      url: "https://example.com/a.txt",
      suggestedFilename: "a.txt",
    });
    const download = await downloadPromise;
    const savePromise = download.saveAs("sub/a.txt");

    const determining = emitDetermining({
      id: downloadId,
      url: "https://example.com/a.txt",
      finalUrl: "https://example.com/a.txt",
      filename: "a.txt",
    });
    expect(determining.isAsync).toBe(false);
    expect(determining.suggestion()).toStrictEqual({
      filename: "sub/a.txt",
      conflictAction: "overwrite",
    });

    emitChanged({
      id: downloadId,
      filename: { current: "/home/user/Downloads/sub/a.txt" },
      state: { current: "complete" },
    });
    await savePromise;
    expect(await download.path()).toBe("/home/user/Downloads/sub/a.txt");
  });

  test("rejects invalid saveAs paths and duplicate saveAs calls", async () => {
    const { Page } = await import("../../lib/page.js");
    const tabId = nextTabId++;
    const downloadId = nextDownloadId++;
    const page = new Page(tabId, Number.POSITIVE_INFINITY);

    const downloadPromise = page.waitForEvent("download");
    emitDebuggerEvent(tabId, "Page.downloadWillBegin", {
      guid: `guid-${downloadId}`,
      url: "https://example.com/invalid.txt",
      suggestedFilename: "invalid.txt",
    });
    const determining = emitDetermining({
      id: downloadId,
      url: "https://example.com/invalid.txt",
      finalUrl: "https://example.com/invalid.txt",
      filename: "invalid.txt",
    });
    const download = await downloadPromise;

    for (const invalid of [
      "",
      "/abs/path.txt",
      "\\abs\\path.txt",
      "C:\\path.txt",
      "../escape.txt",
      "sub/../escape.txt",
      "./current.txt",
      "dir/",
    ]) {
      await expect(download.saveAs(invalid)).rejects.toThrow("relative path");
    }

    const savePromise = download.saveAs("valid.txt");
    await determining.waitSuggested();
    await expect(download.saveAs("second.txt")).rejects.toThrow(
      "already been determined",
    );

    emitChanged({
      id: downloadId,
      filename: { current: "/home/user/Downloads/valid.txt" },
      state: { current: "complete" },
    });
    await savePromise;
  });

  test("matches concurrent downloads across multiple tabs", async () => {
    const { Page } = await import("../../lib/page.js");
    const firstTab = nextTabId++;
    const secondTab = nextTabId++;
    const firstId = nextDownloadId++;
    const secondId = nextDownloadId++;
    const firstPage = new Page(firstTab, Number.POSITIVE_INFINITY);
    const secondPage = new Page(secondTab, Number.POSITIVE_INFINITY);

    const firstDownloadPromise = firstPage.waitForEvent("download");
    const secondDownloadPromise = secondPage.waitForEvent("download");

    emitDebuggerEvent(firstTab, "Page.downloadWillBegin", {
      guid: `guid-${firstId}`,
      url: "https://example.com/export",
      suggestedFilename: "export.csv",
    });
    emitCreated({
      id: firstId,
      url: "https://example.com/export",
      finalUrl: "https://example.com/export",
    });
    emitDebuggerEvent(secondTab, "Page.downloadWillBegin", {
      guid: `guid-${secondId}`,
      url: "https://example.com/export",
      suggestedFilename: "export.csv",
    });
    emitCreated({
      id: secondId,
      url: "https://example.com/export",
      finalUrl: "https://example.com/export",
    });

    const secondDetermining = emitDetermining({
      id: secondId,
      url: "https://example.com/export",
      finalUrl: "https://example.com/export",
      filename: "export.csv",
    });
    const firstDetermining = emitDetermining({
      id: firstId,
      url: "https://example.com/export",
      finalUrl: "https://example.com/export",
      filename: "export.csv",
    });

    const [firstDownload, secondDownload] = await Promise.all([
      firstDownloadPromise,
      secondDownloadPromise,
    ]);

    const firstSave = firstDownload.saveAs("tab1/out.csv");
    const secondSave = secondDownload.saveAs("tab2/out.csv");
    await Promise.all([
      firstDetermining.waitSuggested(),
      secondDetermining.waitSuggested(),
    ]);

    expect(firstDetermining.suggestion()).toStrictEqual({
      filename: "tab1/out.csv",
      conflictAction: "overwrite",
    });
    expect(secondDetermining.suggestion()).toStrictEqual({
      filename: "tab2/out.csv",
      conflictAction: "overwrite",
    });

    emitChanged({
      id: firstId,
      filename: { current: "/home/user/Downloads/tab1/out.csv" },
      state: { current: "complete" },
    });
    emitChanged({
      id: secondId,
      filename: { current: "/home/user/Downloads/tab2/out.csv" },
      state: { current: "complete" },
    });
    await Promise.all([firstSave, secondSave]);
  });

  test("cancels and deletes downloads", async () => {
    const { Page } = await import("../../lib/page.js");
    const tabId = nextTabId++;
    const cancelId = nextDownloadId++;
    const deleteId = nextDownloadId++;
    const page = new Page(tabId, Number.POSITIVE_INFINITY);

    const cancelPromise = page.waitForEvent("download");
    emitDebuggerEvent(tabId, "Page.downloadWillBegin", {
      guid: `guid-${cancelId}`,
      url: "https://example.com/cancel.bin",
      suggestedFilename: "cancel.bin",
    });
    const cancelDetermining = emitDetermining({
      id: cancelId,
      url: "https://example.com/cancel.bin",
      finalUrl: "https://example.com/cancel.bin",
      filename: "cancel.bin",
    });
    const cancelDownload = await cancelPromise;
    await cancelDownload.cancel();
    await cancelDetermining.waitSuggested();
    expect(cancelledIds).toStrictEqual([cancelId]);
    expect(await cancelDownload.failure()).toBe("canceled");
    await expect(cancelDownload.path()).rejects.toThrow("canceled");
    await cancelDownload.cancel();

    const deletePromise = page.waitForEvent("download");
    emitDebuggerEvent(tabId, "Page.downloadWillBegin", {
      guid: `guid-${deleteId}`,
      url: "https://example.com/delete.bin",
      suggestedFilename: "delete.bin",
    });
    const deleteDetermining = emitDetermining({
      id: deleteId,
      url: "https://example.com/delete.bin",
      finalUrl: "https://example.com/delete.bin",
      filename: "delete.bin",
    });
    const deleteDownload = await deletePromise;
    const deletion = deleteDownload.delete();
    await deleteDetermining.waitSuggested();
    emitChanged({
      id: deleteId,
      filename: { current: "/home/user/Downloads/delete.bin" },
      state: { current: "complete" },
    });
    await deletion;
    expect(removedIds).toStrictEqual([deleteId]);
  });

  test("passes through uncontrolled downloads and releases on detach or unmatched predicate", async () => {
    const { Page } = await import("../../lib/page.js");
    const tabId = nextTabId++;
    const page = new Page(tabId, Number.POSITIVE_INFINITY);

    const unrelated = emitDetermining({
      id: nextDownloadId++,
      url: "https://unrelated.example/file.zip",
      finalUrl: "https://unrelated.example/file.zip",
      filename: "file.zip",
    });
    expect(unrelated.isAsync).toBe(false);
    expect(unrelated.suggested()).toBe(true);
    expect(unrelated.suggestion()).toBeUndefined();

    const uncontrolledId = nextDownloadId++;
    emitDebuggerEvent(tabId, "Page.downloadWillBegin", {
      guid: `guid-${uncontrolledId}`,
      url: "https://example.com/no-listener.txt",
      suggestedFilename: "no-listener.txt",
    });
    const uncontrolled = emitDetermining({
      id: uncontrolledId,
      url: "https://example.com/no-listener.txt",
      finalUrl: "https://example.com/no-listener.txt",
      filename: "no-listener.txt",
    });
    expect(uncontrolled.isAsync).toBe(false);
    expect(uncontrolled.suggested()).toBe(true);

    const skippedId = nextDownloadId++;
    const wantedId = nextDownloadId++;
    const waitPromise = page.waitForEvent(
      "download",
      (download: { suggestedFilename: () => string }) =>
        download.suggestedFilename() === "wanted.txt",
    );
    emitDebuggerEvent(tabId, "Page.downloadWillBegin", {
      guid: `guid-${skippedId}`,
      url: "https://example.com/skipped.txt",
      suggestedFilename: "skipped.txt",
    });
    const skippedDetermining = emitDetermining({
      id: skippedId,
      url: "https://example.com/skipped.txt",
      finalUrl: "https://example.com/skipped.txt",
      filename: "skipped.txt",
    });
    await skippedDetermining.waitSuggested();
    expect(skippedDetermining.suggestion()).toBeUndefined();

    emitDebuggerEvent(tabId, "Page.downloadWillBegin", {
      guid: `guid-${wantedId}`,
      url: "https://example.com/wanted.txt",
      suggestedFilename: "wanted.txt",
    });
    const wantedDetermining = emitDetermining({
      id: wantedId,
      url: "https://example.com/wanted.txt",
      finalUrl: "https://example.com/wanted.txt",
      filename: "wanted.txt",
    });
    await waitPromise;
    expect(wantedDetermining.suggested()).toBe(false);

    emitDetach(tabId);
    await wantedDetermining.waitSuggested();
    expect(wantedDetermining.suggestion()).toBeUndefined();
  });
});
