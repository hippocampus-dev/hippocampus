import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { h } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import type { VoiceInputCandidate } from "../ipc/types/voice_input.gen";

type VoiceInputState =
  | "idle"
  | "starting"
  | "recording"
  | "processing"
  | "reviewing";

const VoiceIndicator = ({}) => {
  const [state, setState] = useState<VoiceInputState>("starting");
  const [candidate, setCandidate] = useState<VoiceInputCandidate | null>(null);

  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const removeEventListeners: UnlistenFn[] = [];

    (async () => {
      removeEventListeners.push(
        await listen<string>("voice-input-state", (event) => {
          setState(event.payload as VoiceInputState);
          if (event.payload === "idle") {
            setCandidate(null);
          }
        }),
      );
      removeEventListeners.push(
        await listen<VoiceInputCandidate>("voice-input-candidate", (event) => {
          setCandidate(event.payload);
        }),
      );
    })();

    return () => {
      removeEventListeners.forEach((removeEventListener) =>
        removeEventListener(),
      );
    };
  }, []);

  useEffect(() => {
    (async () => {
      const current = getCurrentWindow();
      await current.setSize(
        new LogicalSize(ref.current?.offsetWidth!, ref.current?.offsetHeight!),
      );
    })();
  }, [state, candidate]);

  const label =
    state === "starting"
      ? "Starting..."
      : state === "recording"
        ? "Recording..."
        : state === "processing"
          ? "Processing..."
          : state === "reviewing"
            ? candidate?.revisable
              ? "y: accept / n: reject / r: revise"
              : "y: accept / n: reject"
            : "";

  const stoppable = state === "starting" || state === "recording";

  const handleClick = async () => {
    if (stoppable) {
      await invoke("stop_voice_input");
    }
  };

  return h(
    "div",
    {
      ref: ref,
      class: `flex flex-col justify-center gap-2 w-max min-w-[200px] max-w-[400px] min-h-[56px] bg-gray-800 px-4 py-2 m-0 box-border ${stoppable ? "cursor-pointer" : ""}`,
      onClick: handleClick,
    },
    [
      candidate === null
        ? null
        : h(
            "p",
            { class: "text-white text-sm whitespace-pre-wrap break-words" },
            candidate.text,
          ),
      h("div", { class: "flex items-center justify-center gap-2" }, [
        state === "recording" &&
          h("div", {
            class:
              "w-3 h-3 bg-red-500 rounded-full animate-[recording-pulse_1.5s_infinite]",
          }),
        state === "processing" &&
          h("div", {
            class:
              "w-4 h-4 border-2 border-gray-300 border-t-blue-500 rounded-full animate-spin",
          }),
        h(
          "span",
          {
            class: `text-sm ${state === "reviewing" ? "text-gray-400" : "text-white"}`,
          },
          label,
        ),
      ]),
    ],
  );
};

export default VoiceIndicator;
