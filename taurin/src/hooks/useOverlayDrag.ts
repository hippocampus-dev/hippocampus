import { invoke } from "@tauri-apps/api/core";
import type { UnlistenFn } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { useEffect } from "preact/hooks";

const POSITION_SAVE_DELAY = 500;

const useOverlayDrag = () => {
  useEffect(() => {
    let timer = 0;
    let saved: string | null = null;
    let removeEventListener: UnlistenFn | undefined;

    (async () => {
      // startDragging hands the drag to the window manager, so a settled run of moves is what marks its end
      removeEventListener = await getCurrentWindow().onMoved(({ payload }) => {
        const moved = `${payload.x},${payload.y}`;
        // GTK reports a move for every configure event, resizes included, and the first one is the position the overlay was shown at
        if (saved === null || saved === moved) {
          saved = moved;
          return;
        }
        saved = moved;

        window.clearTimeout(timer);
        timer = window.setTimeout(async () => {
          await invoke("save_overlay_position", {
            position: { x: payload.x, y: payload.y },
          });
        }, POSITION_SAVE_DELAY);
      });
    })();

    return () => {
      window.clearTimeout(timer);
      removeEventListener?.();
    };
  }, []);

  return (event: MouseEvent) => {
    if (event.button === 0) getCurrentWindow().startDragging();
  };
};

export default useOverlayDrag;
