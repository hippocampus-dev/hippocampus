import { ArrowLeftIcon } from "@heroicons/react/24/outline";
import { Channel, invoke } from "@tauri-apps/api/core";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { message, open, save } from "@tauri-apps/plugin-dialog";
import { h } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import { Link } from "preact-router";
import { describeCommandError, isCancelled } from "../ipc/error.ts";
import type {
  CsvTable,
  RepositoryCreationEvent,
} from "../ipc/types/create_github_repositories.ts";

const COLUMNS = ["repository", "description", "visibility"];
const REQUIRED_COLUMNS = ["repository"];
const RESULT_COLUMNS = ["html_url"];

interface Result {
  status: "pending" | "running" | "done" | "failed";
  values: Record<string, string>;
  error: string;
}

const STATUS_CLASSES = {
  pending: "",
  running: "text-blue-600",
  done: "text-green-600",
  failed: "text-red-600",
};

const pendingResults = (table: CsvTable | null): Result[] =>
  (table?.records ?? []).map(() => ({
    status: "pending",
    values: {},
    error: "",
  }));

const CreateGitHubRepositories = ({ file: initialFile }: { file?: string }) => {
  const [file, setFile] = useState(initialFile ?? "");
  const [table, setTable] = useState<CsvTable | null>(null);
  const [results, setResults] = useState<Result[]>([]);
  const [concurrency, setConcurrency] = useState("4");
  const [isRunning, setIsRunning] = useState(false);
  const [isFinished, setIsFinished] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    (async () => {
      const current = getCurrentWindow();
      await current.setSize(
        new LogicalSize(ref.current?.offsetWidth!, ref.current?.offsetHeight!),
      );
    })();
  }, [table]);

  const load = async (path: string) => {
    setFile(path);
    try {
      const table: CsvTable = await invoke("read_github_repositories", {
        path,
      });
      setTable(table);
      setResults(pendingResults(table));
      setIsFinished(false);
    } catch (e) {
      setTable(null);
      await message(describeCommandError(e));
    }
  };

  useEffect(() => {
    if (initialFile !== undefined) {
      load(initialFile);
    }
  }, []);

  const handleSelect = async () => {
    const selected = await open({
      filters: [{ name: "CSV", extensions: ["csv"] }],
    });
    if (selected !== null) {
      await load(selected);
    }
  };

  const handleRun = async () => {
    setResults(pendingResults(table));
    setIsRunning(true);
    setIsFinished(false);
    try {
      const eventChannel = new Channel<RepositoryCreationEvent>();
      eventChannel.onmessage = (event) => {
        setResults((results) =>
          results.map((result, index) => {
            if (index !== event.index) return result;
            if (event.kind === "Running") {
              return { ...result, status: "running" };
            }
            if (event.kind === "Created") {
              return {
                ...result,
                status: "done",
                values: { html_url: event.html_url },
              };
            }
            return { ...result, status: "failed", error: event.message };
          }),
        );
      };
      await invoke("create_github_repositories", {
        header: table?.header,
        records: table?.records,
        concurrency: Math.max(1, Math.trunc(Number(concurrency))),
        eventTx: eventChannel,
      });
    } catch (e) {
      if (isCancelled(e)) {
        setResults((results) =>
          results.map((result) =>
            result.status === "running"
              ? { ...result, status: "failed", error: "cancelled" }
              : result,
          ),
        );
      } else {
        await message(describeCommandError(e));
      }
    } finally {
      setIsRunning(false);
      setIsFinished(true);
    }
  };

  const handleCancel = async () => {
    await invoke("cancel_create_github_repositories");
  };

  const handleDownload = async () => {
    if (table === null) return;
    const path = await save({
      defaultPath: file.replace(/(\.csv)?$/i, "-result.csv"),
      filters: [{ name: "CSV", extensions: ["csv"] }],
    });
    if (path === null) return;
    try {
      await invoke("save_github_repositories_result", {
        path,
        table: [
          [...table.header, ...RESULT_COLUMNS, "error"],
          ...table.records.map((record, index) => [
            ...record,
            ...RESULT_COLUMNS.map((name) => results[index].values[name] ?? ""),
            results[index].error,
          ]),
        ],
      });
    } catch (e) {
      await message(describeCommandError(e));
    }
  };

  const missing = REQUIRED_COLUMNS.filter(
    (name) => !(table?.header ?? []).includes(name),
  );
  const failed = table !== null && missing.length > 0;

  return h(
    "div",
    {
      ref,
      class: "flex flex-col items-center bg-gray-100 p-6 space-y-4 h-full",
    },
    [
      h("div", { class: "flex items-center w-full lg:w-1/2 mt-4" }, [
        h(
          Link,
          {
            href: "/",
            class: `text-gray-600 hover:text-gray-900 ${isRunning ? "invisible" : ""}`,
            "data-native": "",
          },
          [h(ArrowLeftIcon, { class: "size-6", "aria-hidden": "true" })],
        ),
        h(
          "h1",
          { class: "ml-4 text-xl font-bold" },
          "Create GitHub Repositories",
        ),
      ]),
      h("div", { class: "w-full lg:w-1/2 mt-4 space-y-2" }, [
        h("div", { class: "grid grid-cols-[120px_1fr] items-center gap-2" }, [
          h("span", {}, "CSV"),
          h("div", { class: "flex items-center gap-2" }, [
            h(
              "button",
              {
                onClick: handleSelect,
                disabled: isRunning,
                class:
                  "bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed",
              },
              "Select",
            ),
            h("span", { class: "text-gray-600 break-all" }, file),
          ]),
        ]),
        h(
          "p",
          { class: failed ? "text-red-600" : "" },
          failed
            ? `CSV header must include: ${missing.join(", ")} (found: ${table.header.map((name) => JSON.stringify(name)).join(", ")})`
            : `Columns: ${COLUMNS.join(", ")}`,
        ),
      ]),
      table !== null &&
        h("div", { class: "w-full lg:w-1/2 space-y-2" }, [
          h("div", { class: "grid grid-cols-[120px_1fr] items-center gap-2" }, [
            h("span", {}, "Rows"),
            h("span", {}, String(table.records.length)),
          ]),
          h("div", { class: "grid grid-cols-[120px_1fr] items-center gap-2" }, [
            h("label", { for: "concurrency" }, "Concurrency"),
            h("input", {
              id: "concurrency",
              type: "number",
              min: 1,
              value: concurrency,
              onInput: (event: Event) =>
                setConcurrency((event.target as HTMLInputElement).value),
              class: "rounded-md border border-gray-300 px-2 py-1",
            }),
          ]),
          h("div", { class: "flex gap-2" }, [
            h(
              "button",
              {
                onClick: handleRun,
                disabled: missing.length > 0 || isRunning,
                class:
                  "bg-green-600 hover:bg-green-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed",
              },
              "Run",
            ),
            h(
              "button",
              {
                onClick: handleDownload,
                disabled: !isFinished,
                class:
                  "bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed",
              },
              "Download",
            ),
            h(
              "button",
              {
                onClick: handleCancel,
                disabled: !isRunning,
                class:
                  "bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed",
              },
              "Cancel",
            ),
          ]),
        ]),
      table !== null &&
        h(
          "div",
          { class: "w-full lg:w-1/2 max-h-64 overflow-auto" },
          h("table", { class: "border-collapse bg-white" }, [
            h(
              "thead",
              {},
              h(
                "tr",
                {},
                [...table.header, ...RESULT_COLUMNS, "status"].map((name) =>
                  h(
                    "th",
                    { class: "border border-gray-300 px-2 py-1 text-left" },
                    name,
                  ),
                ),
              ),
            ),
            h(
              "tbody",
              {},
              table.records.map((record, index) =>
                h("tr", {}, [
                  ...record.map((cell) =>
                    h(
                      "td",
                      {
                        class:
                          "border border-gray-300 px-2 py-1 align-top max-w-xs break-words",
                      },
                      cell,
                    ),
                  ),
                  ...RESULT_COLUMNS.map((name) =>
                    h(
                      "td",
                      {
                        class:
                          "border border-gray-300 px-2 py-1 align-top max-w-xs break-words",
                      },
                      results[index]?.values[name] ?? "",
                    ),
                  ),
                  h(
                    "td",
                    {
                      class: `border border-gray-300 px-2 py-1 align-top max-w-xs break-words ${STATUS_CLASSES[results[index]?.status ?? "pending"]}`,
                    },
                    results[index]?.error || results[index]?.status,
                  ),
                ]),
              ),
            ),
          ]),
        ),
    ],
  );
};

export default CreateGitHubRepositories;
