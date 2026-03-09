import {h} from "preact";
import {useState, useEffect} from "preact/hooks";
import {listen, UnlistenFn} from "@tauri-apps/api/event";
import {invoke} from "@tauri-apps/api/core";

type VoiceInputState = "idle" | "starting" | "recording" | "processing";

const VoiceIndicator = ({}) => {
    const [state, setState] = useState<VoiceInputState>("starting");

    useEffect(() => {
        const removeEventListeners: UnlistenFn[] = [];

        (async () => {
            removeEventListeners.push(await listen<string>("voice-input-state", (event) => {
                setState(event.payload as VoiceInputState);
            }));
        })();

        return () => {
            removeEventListeners.forEach((removeEventListener) => removeEventListener());
        };
    }, []);

    const label = state === "starting" ? "Starting..."
        : state === "recording" ? "Recording..."
        : state === "processing" ? "Processing..."
        : "";

    const stoppable = state === "starting" || state === "recording";

    const handleClick = async () => {
        if (stoppable) {
            await invoke("stop_voice_input");
        }
    };

    return (
        h("div", {class: `flex items-center justify-center gap-2 w-screen h-screen bg-gray-800 px-4 m-0 box-border ${stoppable ? "cursor-pointer" : ""}`, onClick: handleClick}, [
            state === "recording" && h("div", {class: "w-3 h-3 bg-red-500 rounded-full animate-[recording-pulse_1.5s_infinite]"}),
            state === "processing" && h("div", {class: "w-4 h-4 border-2 border-gray-300 border-t-blue-500 rounded-full animate-spin"}),
            h("span", {class: "text-white text-sm"}, label),
        ])
    );
};

export default VoiceIndicator;
