// https://heroicons.com/
import {
  Cog8ToothIcon,
  ComputerDesktopIcon,
  LanguageIcon,
  MicrophoneIcon,
} from "@heroicons/react/24/solid";
import { invoke } from "@tauri-apps/api/core";
import { listen, TauriEvent, type UnlistenFn } from "@tauri-apps/api/event";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { message } from "@tauri-apps/plugin-dialog";
import { type ComponentType, h } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import { Link, route } from "preact-router";

import LoadingOverlay from "../components/LoadingOverlay.tsx";
import { describeCommandError, isCancelled } from "../ipc/error.ts";

interface LineItem {
  icon?: ComponentType<any>;
  label: string;
  handler: (abortController: AbortController) => Promise<string | undefined>;
  abortHandler?: (abortController: AbortController) => Promise<void>;
  background?: boolean;
}

const lines: LineItem[] = [
  {
    icon: ComputerDesktopIcon,
    label: "Explain Monitors",
    handler: async () => {
      try {
        const explanation: string = await invoke("explain_monitors");
        return explanation;
      } catch (e) {
        return isCancelled(e) ? undefined : describeCommandError(e);
      }
    },
    abortHandler: async () => {
      await invoke("cancel_explain_monitors");
    },
    background: true,
  },
  {
    icon: LanguageIcon,
    label: "Realtime Translation",
    handler: async () => {
      route("/translation");
      return undefined;
    },
  },
  {
    icon: MicrophoneIcon,
    label: "Voice Input",
    handler: async () => {
      await getCurrentWindow().hide();
      try {
        await invoke("start_voice_input");
      } catch (e) {
        await getCurrentWindow().show();
      }
      return undefined;
    },
  },
];
if (import.meta.env.DEV) {
  lines.push(
    {
      label: "value1",
      handler: async (abortController: AbortController) => {
        try {
          const response = await fetch("https://httpbin.org/ip", {
            signal: abortController.signal,
          });
          return JSON.stringify(await response.json(), null, 2);
        } catch (e) {
          return String(e);
        }
      },
      abortHandler: async (abortController: AbortController) => {
        abortController.abort();
      },
    },
    {
      label: "value2",
      handler: async () => {
        try {
          const cookie: string = await invoke("bake");
          return cookie;
        } catch (e) {
          return isCancelled(e) ? undefined : describeCommandError(e);
        }
      },
      abortHandler: async () => {
        await invoke("cancel_bake");
      },
    },
    { label: "value3", handler: async () => "value3" },
    { label: "value4", handler: async () => "value4" },
    { label: "value5", handler: async () => "value5" },
    { label: "value6", handler: async () => "value6" },
    { label: "value7", handler: async () => "value7" },
    { label: "value8", handler: async () => "value8" },
    { label: "value9", handler: async () => "value9" },
    { label: "value10", handler: async () => "value10" },
    { label: "value11", handler: async () => "value11" },
    { label: "value12", handler: async () => "value12" },
    { label: "value13", handler: async () => "value13" },
    { label: "value14", handler: async () => "value14" },
    { label: "value15", handler: async () => "value15" },
    { label: "value16", handler: async () => "value16" },
    { label: "value17", handler: async () => "value17" },
    { label: "value18", handler: async () => "value18" },
    { label: "value19", handler: async () => "value19" },
    { label: "value20", handler: async () => "value20" },
  );
}

const Index = ({}) => {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [abortController, setAbortController] = useState<AbortController>(
    new AbortController(),
  );
  const [activeItem, setActiveItem] = useState<LineItem | null>(null);

  const ref = useRef<HTMLDivElement | null>(null);
  const searchBoxRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    (async () => {
      const current = getCurrentWindow();
      await current.setSize(
        new LogicalSize(ref.current?.offsetWidth!, ref.current?.offsetHeight!),
      );
    })();
  }, [query]);

  const cancelLoading = async () => {
    if (!isLoading) return;
    const item = activeItem;
    abortController.abort();
    setIsLoading(false);
    setAbortController(new AbortController());
    setActiveItem(null);
    if (item?.abortHandler) {
      await item.abortHandler(abortController);
    }
    if (item?.background) {
      await getCurrentWindow().show();
    }
  };

  const filteredLines = lines.filter((line) =>
    line.label.toLowerCase().includes(query.toLowerCase()),
  );

  const executeItem = async (item: LineItem) => {
    setIsLoading(true);
    setActiveItem(item);

    if (item.background) {
      await getCurrentWindow().hide();
    }

    let result: string | undefined;
    try {
      result = await item.handler(abortController);
    } finally {
      setIsLoading(false);
      setAbortController(new AbortController());
      setActiveItem(null);
    }

    if (abortController.signal.aborted) {
      return;
    }

    if (result !== undefined) {
      await message(result);
    }

    if (item.background) {
      await getCurrentWindow().show();
    }
  };

  const handleKeyDown = async (e: KeyboardEvent) => {
    if (filteredLines.length === 0) {
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((i) => (i === 0 ? filteredLines.length - 1 : i - 1));
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((i) => (i === filteredLines.length - 1 ? 0 : i + 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      await executeItem(filteredLines[selectedIndex]);
    } else if (e.ctrlKey && e.key === "c") {
      e.preventDefault();
      if (isLoading) {
        await cancelLoading();
      } else if (filteredLines[selectedIndex]?.abortHandler) {
        await filteredLines[selectedIndex]?.abortHandler(abortController);
      }
    } else if (e.key === "Escape") {
      e.preventDefault();
      await getCurrentWindow().hide();
    }
  };

  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  useEffect(() => {
    const handleFocus = () => {
      searchBoxRef.current?.focus();
    };

    const removeEventListeners: UnlistenFn[] = [];

    (async () => {
      removeEventListeners.push(
        await listen(TauriEvent.WINDOW_FOCUS, handleFocus),
      );
    })();

    return () => {
      removeEventListeners.forEach((removeEventListener) =>
        removeEventListener(),
      );
    };
  }, []);

  return h(
    "div",
    {
      ref: ref,
      class: "flex flex-col items-center bg-gray-100 p-6 space-y-4 h-full",
      tabIndex: 0,
      onKeyDown: handleKeyDown,
    },
    [
      isLoading && h(LoadingOverlay, { onCancel: cancelLoading }),
      h("div", { class: "flex items-center w-full lg:w-1/2 mt-4" }, [
        h(
          "label",
          { for: "search-box", class: "sr-only" },
          "Search for given file",
        ),
        h("input", {
          ref: searchBoxRef,
          type: "text",
          value: query,
          placeholder: "Search for given file",
          id: "search-box",
          onInput: (event) =>
            setQuery((event.target as HTMLInputElement).value),
          autofocus: true,
          class:
            "w-full rounded-md border-4 border-orange-600 focus:ring-4 focus:ring-orange-600 p-4",
        }),
        h(
          Link,
          {
            href: "/settings",
            class: "ml-4 text-gray-600 hover:text-gray-900",
            "data-native": "",
          },
          [h(Cog8ToothIcon, { class: "size-6", "aria-hidden": "true" })],
        ),
      ]),
      h("div", { class: "w-full max-h-100 overflow-y-auto" }, [
        h("table", { class: "min-w-full" }, [
          h(
            "tbody",
            { class: "bg-white divide-y divide-orange-600" },
            filteredLines.map((line, i) =>
              h(
                "tr",
                {
                  key: line.label,
                  onClick: async () => {
                    setSelectedIndex(i);
                    await executeItem(line);
                  },
                  class: `hover:bg-gray-100 ${i === selectedIndex ? "bg-gray-200" : ""}`,
                },
                [
                  h(
                    "td",
                    { class: "px-6 py-4 whitespace-nowrap flex items-center" },
                    [
                      line.icon
                        ? h(line.icon, {
                            class: "size-6 mr-2",
                            "aria-hidden": "true",
                          })
                        : null,
                      h("span", {}, line.label),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ]),
      ]),
    ],
  );
};

export default Index;
