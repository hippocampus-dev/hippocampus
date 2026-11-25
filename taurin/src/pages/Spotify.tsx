import { MusicalNoteIcon, PauseIcon } from "@heroicons/react/24/solid";
import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import useOverlayDrag from "../hooks/useOverlayDrag.ts";
import { describeCommandError } from "../ipc/error.ts";
import type { CurrentlyPlaying } from "../ipc/types/spotify.gen";

const POLL_INTERVAL = 10000;
const PROGRESS_TICK_INTERVAL = 1000;
const MAX_RETRIES = 60;
const RETRY_INTERVAL = 60000;

const formatDuration = (milliseconds: number): string => {
  const totalSeconds = Math.floor(milliseconds / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
};

const Spotify = ({}) => {
  const [currentlyPlaying, setCurrentlyPlaying] =
    useState<CurrentlyPlaying | null>(null);
  const [progressMilliseconds, setProgressMilliseconds] = useState<
    number | null
  >(null);
  const [error, setError] = useState<string | null>(null);
  const [failedImageUrl, setFailedImageUrl] = useState<string | null>(null);

  const onMouseDown = useOverlayDrag();

  useEffect(() => {
    let timer = 0;
    let stopped = false;
    let retries = 0;
    let exhausted = false;
    let removeEventListener: UnlistenFn | undefined;

    const poll = async () => {
      let failed = false;
      try {
        const playing = await invoke<CurrentlyPlaying | null>(
          "get_spotify_currently_playing",
        );
        setCurrentlyPlaying(playing);
        setProgressMilliseconds(
          playing === null ? null : playing.progress_milliseconds,
        );
        setError(null);
      } catch (e) {
        setCurrentlyPlaying(null);
        setProgressMilliseconds(null);
        setError(describeCommandError(e));
        failed = true;
      }

      if (stopped) return;
      if (!failed) {
        retries = 0;
        timer = window.setTimeout(poll, POLL_INTERVAL);
      } else if (
        retries < MAX_RETRIES &&
        // Retrying without a token would open the browser for sign-in again
        (await invoke<boolean>("has_spotify_token"))
      ) {
        retries += 1;
        timer = window.setTimeout(poll, RETRY_INTERVAL);
      } else {
        exhausted = true;
      }
    };

    (async () => {
      removeEventListener = await listen("spotify-enabled", () => {
        if (!exhausted) return;
        exhausted = false;
        retries = 0;
        poll();
      });
    })();

    poll();

    return () => {
      stopped = true;
      window.clearTimeout(timer);
      removeEventListener?.();
    };
  }, []);

  useEffect(() => {
    if (
      currentlyPlaying === null ||
      !currentlyPlaying.is_playing ||
      currentlyPlaying.progress_milliseconds === null
    ) {
      return;
    }

    const startedAt = Date.now();
    const baseProgress = currentlyPlaying.progress_milliseconds;
    const duration = currentlyPlaying.duration_milliseconds;
    const interval = window.setInterval(() => {
      setProgressMilliseconds(
        Math.min(baseProgress + (Date.now() - startedAt), duration),
      );
    }, PROGRESS_TICK_INTERVAL);

    return () => {
      window.clearInterval(interval);
    };
  }, [currentlyPlaying]);

  const progressPercentage =
    currentlyPlaying !== null &&
    currentlyPlaying.duration_milliseconds > 0 &&
    progressMilliseconds !== null
      ? Math.min(
          100,
          (progressMilliseconds / currentlyPlaying.duration_milliseconds) * 100,
        )
      : 0;

  return h(
    "div",
    {
      class:
        "w-screen h-screen bg-gray-800 p-3 m-0 box-border cursor-move select-none flex items-center gap-3 overflow-hidden",
      onMouseDown,
    },
    error !== null
      ? [
          h(
            "span",
            { class: "text-white text-sm line-clamp-3 break-words" },
            error,
          ),
        ]
      : currentlyPlaying === null
        ? []
        : [
            h(
              "div",
              {
                class:
                  "relative size-18 shrink-0 rounded overflow-hidden bg-gray-700 flex items-center justify-center",
              },
              [
                currentlyPlaying.image_url !== null &&
                failedImageUrl !== currentlyPlaying.image_url
                  ? h("img", {
                      src: currentlyPlaying.image_url,
                      alt:
                        currentlyPlaying.album !== ""
                          ? currentlyPlaying.album
                          : currentlyPlaying.track,
                      draggable: false,
                      onError: () =>
                        setFailedImageUrl(currentlyPlaying.image_url),
                      class: "size-full object-cover",
                    })
                  : h(MusicalNoteIcon, {
                      class: "size-8 text-gray-400",
                      "aria-hidden": "true",
                    }),
                !currentlyPlaying.is_playing &&
                  h(
                    "div",
                    {
                      class:
                        "absolute inset-0 bg-black/50 flex items-center justify-center",
                    },
                    [
                      h(PauseIcon, {
                        class: "size-7 text-white",
                        "aria-hidden": "true",
                      }),
                    ],
                  ),
              ],
            ),
            h(
              "div",
              { class: "min-w-0 flex-1 h-full flex flex-col justify-between" },
              [
                h("div", { class: "min-w-0" }, [
                  h(
                    "div",
                    {
                      class:
                        "text-white text-base font-semibold leading-5 truncate",
                    },
                    currentlyPlaying.track,
                  ),
                  h(
                    "div",
                    { class: "text-gray-300 text-xs leading-4 truncate" },
                    currentlyPlaying.artists,
                  ),
                  currentlyPlaying.album !== "" &&
                    h(
                      "div",
                      { class: "text-gray-400 text-xs leading-4 truncate" },
                      currentlyPlaying.album,
                    ),
                ]),
                h("div", { class: "flex items-center gap-2" }, [
                  h(
                    "span",
                    {
                      class:
                        "text-gray-400 text-xs tabular-nums shrink-0 leading-none",
                    },
                    progressMilliseconds === null
                      ? "--:--"
                      : formatDuration(progressMilliseconds),
                  ),
                  h(
                    "div",
                    {
                      class:
                        "flex-1 h-1.5 bg-gray-600 rounded-full overflow-hidden",
                    },
                    [
                      h("div", {
                        class: `h-full rounded-full ${currentlyPlaying.is_playing ? "bg-green-500" : "bg-gray-400"}`,
                        style: { width: `${progressPercentage}%` },
                      }),
                    ],
                  ),
                  h(
                    "span",
                    {
                      class:
                        "text-gray-400 text-xs tabular-nums shrink-0 leading-none",
                    },
                    formatDuration(currentlyPlaying.duration_milliseconds),
                  ),
                ]),
              ],
            ),
          ],
  );
};

export default Spotify;
