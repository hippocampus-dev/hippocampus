import { Link } from "preact-router";
import { h } from "preact";
import { useState, useEffect, useRef } from "preact/hooks";
import { getCurrentWindow, PhysicalSize } from "@tauri-apps/api/window";
import { invoke, Channel } from "@tauri-apps/api/core";
import { ArrowLeftIcon } from "@heroicons/react/24/outline";

const Translation = ({}) => {
    const [isRunning, setIsRunning] = useState(false);
    const [content, setContent] = useState<string>("");
    const ref = useRef<HTMLDivElement | null>(null);
    const contentContainerRef = useRef<HTMLDivElement | null>(null);

    useEffect(() => {
        (async () => {
            const current = getCurrentWindow();
            await current.setSize(new PhysicalSize(ref.current?.offsetWidth!, ref.current?.offsetHeight!));
        })();
    }, []);

    useEffect(() => {
        if (contentContainerRef.current) {
            contentContainerRef.current.scrollTop = contentContainerRef.current.scrollHeight;
        }
    }, [content]);

    const handleStart = async () => {
        if (isRunning) return;

        try {
            const eventChannel = new Channel<string>();

            eventChannel.onmessage = (event) => {
                setContent((content) => {
                    return content + event;
                });
            };

            await invoke("start_realtime_translation", {
                resultTx: eventChannel,
            });

            setIsRunning(true);
        } catch (e) {
            alert(e);
        }
    };

    const handleStop = async () => {
        if (!isRunning) return;

        try {
            await invoke("stop_realtime_translation");

            setIsRunning(false);
        } catch (e) {
            alert(e);
        }
    };

    const handleClear = () => {
        setContent("");
    };

    return (
        h("div", { ref, class: "flex flex-col items-center bg-gray-100 p-6 space-y-4 h-full" }, [
            h("div", { class: "flex items-center w-full lg:w-1/2 mt-4" }, [
                h(Link, { href: "/", class: "text-gray-600 hover:text-gray-900", "data-native": "" }, [
                    h(ArrowLeftIcon, { class: "size-6", "aria-hidden": "true" })
                ]),
                h("h1", { class: "ml-4 text-xl font-bold" }, "Realtime Translation"),
            ]),
            h("div", { class: "w-full lg:w-1/2 mt-4 flex space-x-4" }, [
                h("button", {
                    onClick: handleStart,
                    disabled: isRunning,
                    class: `bg-green-600 hover:bg-green-700 text-white font-bold py-2 px-4 rounded ${isRunning ? 'opacity-50 cursor-not-allowed' : ''}`,
                }, "Start"),
                h("button", {
                    onClick: handleStop,
                    disabled: !isRunning,
                    class: `bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded ${!isRunning ? 'opacity-50 cursor-not-allowed' : ''}`,
                }, "Stop"),
                h("button", {
                    onClick: handleClear,
                    class: "bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded",
                }, "Clear"),
            ]),
            h("div", {
                ref: contentContainerRef,
                class: "w-full lg:w-1/2 mt-4 bg-white p-4 rounded-md shadow-md h-64 overflow-y-auto"
            }, [
                h("h2", { class: "text-lg font-semibold mb-2" }, "Result"),
                content.length === 0
                    ? h("p", { class: "text-gray-500 italic" }, "No content yet. Click Start to begin.")
                    : h("div", {
                        class: "whitespace-pre-wrap break-words font-mono text-medium",
                      }, content)
            ]),
        ])
    );
};

export default Translation;
