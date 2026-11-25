import { invoke } from "@tauri-apps/api/core";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { h } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import useOverlayDrag from "../hooks/useOverlayDrag.ts";
import type { TmuxWindows } from "../ipc/types/tmux_monitor.gen";

const POLL_INTERVAL = 1000;

const TmuxMonitor = ({}) => {
  const [tmuxWindows, setTmuxWindows] = useState<TmuxWindows>([]);

  const ref = useRef<HTMLDivElement | null>(null);

  const onMouseDown = useOverlayDrag();

  useEffect(() => {
    let timer = 0;
    let stopped = false;

    const poll = async () => {
      if (await getCurrentWindow().isVisible()) {
        try {
          const listed = await invoke<TmuxWindows>("list_tmux_windows");
          // Keeping the reference on an unchanged list stops the resize effect from firing every poll
          setTmuxWindows((current) =>
            JSON.stringify(current) === JSON.stringify(listed)
              ? current
              : listed,
          );
        } catch (e) {
          console.error("Failed to list tmux windows:", e);
        }
      }

      if (!stopped) {
        timer = window.setTimeout(poll, POLL_INTERVAL);
      }
    };

    poll();

    return () => {
      stopped = true;
      window.clearTimeout(timer);
    };
  }, []);

  useEffect(() => {
    (async () => {
      const current = getCurrentWindow();
      await current.setSize(
        new LogicalSize(ref.current?.offsetWidth!, ref.current?.offsetHeight!),
      );
    })();
  }, [tmuxWindows]);

  return h(
    "div",
    {
      ref: ref,
      class: "w-screen bg-gray-800 p-2 m-0 box-border cursor-move",
      onMouseDown,
    },
    tmuxWindows.length === 0
      ? [h("span", { class: "text-gray-400 text-sm" }, "No tmux window")]
      : tmuxWindows.map((tmuxWindow) =>
          h(
            "div",
            {
              key: `${tmuxWindow.session_name}:${tmuxWindow.window_index}`,
              class: "flex items-center gap-2 py-0.5",
            },
            [
              h("div", {
                class: `size-2 shrink-0 rounded-full ${tmuxWindow.silent ? "bg-green-500" : "bg-gray-500"}`,
              }),
              h(
                "span",
                { class: "text-gray-400 text-sm shrink-0" },
                `${tmuxWindow.session_name}:${tmuxWindow.window_index}`,
              ),
              h(
                "span",
                { class: "text-white text-sm truncate" },
                tmuxWindow.title,
              ),
            ],
          ),
        ),
  );
};

export default TmuxMonitor;
