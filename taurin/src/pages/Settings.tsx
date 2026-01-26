import {Link} from "preact-router";
import {h} from "preact";
import {useState, useEffect, useRef} from "preact/hooks";
import {getCurrentWindow, LogicalSize} from "@tauri-apps/api/window";
import {getVersion} from "@tauri-apps/api/app";
import {confirm} from "@tauri-apps/plugin-dialog";
import {invoke} from "@tauri-apps/api/core";
import {ArrowLeftIcon} from "@heroicons/react/24/outline";
import {locale} from "@tauri-apps/plugin-os";

import Switch from "../components/Switch.tsx";
import ShortcutSetting from "../components/Shortcut.tsx";
import Select from "../components/Select.tsx";
import {Settings as IPCSettings} from "../ipc/types/settings";

const Settings = ({}) => {
    const [version, setVersion] = useState("");
    const [autoStart, setAutoStart] = useState<boolean | null>(null);
    const [autoUpdate, setAutoUpdate] = useState<boolean | null>(null);
    const [hotKey, setHotKey] = useState<string | null>(null);
    const [realtimeTranslationLanguage, setRealtimeTranslationLanguage] = useState<string | null>(null);

    const ref = useRef<HTMLDivElement | null>(null);

    useEffect(() => {
        (async () => {
            const settings: IPCSettings = await invoke("get_settings");
            settings.forEach((setting) => {
                switch (setting.handler_command) {
                    case "handle_auto_start":
                        setAutoStart(setting.current_value);
                        break;
                    case "handle_auto_update":
                        setAutoUpdate(setting.current_value);
                        break;
                    case "handle_hot_key":
                        setHotKey(setting.current_value);
                        break;
                    case "handle_realtime_translation_language":
                        setRealtimeTranslationLanguage(setting.current_value);
                        break;
                }
            })
        })();
    }, []);

    useEffect(() => {
        (async () => {
            const current = getCurrentWindow();
            await current.setSize(new LogicalSize(ref.current?.offsetWidth!, ref.current?.offsetHeight!));
            const version = await getVersion();
            setVersion(version);
        })();
    }, []);

    useEffect(() => {
        if (autoStart === null) return;
        (async () => {
            await invoke("handle_auto_start", {value: autoStart});
        })();
    }, [autoStart]);

    useEffect(() => {
        if (autoUpdate === null) return;
        (async () => {
            await invoke("handle_auto_update", {value: autoUpdate});
        })();
    }, [autoUpdate]);

    useEffect(() => {
        if (hotKey === null) return;
        (async () => {
            await invoke("handle_hot_key", {value: hotKey});
        })();
    }, [hotKey]);

    useEffect(() => {
        if (realtimeTranslationLanguage === null) return;
        (async () => {
            await invoke("handle_realtime_translation_language", {value: realtimeTranslationLanguage});
        })();
    }, [realtimeTranslationLanguage]);

    return (
        h("div", {ref: ref, class: "flex flex-col items-center bg-gray-100 p-6 space-y-4 h-full"}, [
            h("div", {class: "flex items-center w-full lg:w-1/2 mt-4"}, [
                h(Link, {href: "/", class: "text-gray-600 hover:text-gray-900", "data-native": ""}, [
                    h(ArrowLeftIcon, {class: "size-6", "aria-hidden": "true"})
                ]),
                h("span", {class: "ml-auto text-gray-600"}, version),
            ]),

            h("div", {class: "w-full lg:w-1/2 mt-6"}, [
                h("h2", {class: "text-lg font-semibold mb-4 border-b pb-2"}, "General"),
                h("div", {class: "space-y-4"}, [
                    h("div", {class: "mt-2"}, [
                        h(Switch, {
                            label: "Enable Auto Start",
                            checked: autoStart ?? false,
                            onChange: (value) => setAutoStart(value),
                        }),
                    ]),
                    h("div", {class: "mt-2"}, [
                        h(Switch, {
                            label: "Enable Auto Update",
                            checked: autoUpdate ?? false,
                            onChange: (value) => setAutoUpdate(value),
                        }),
                    ]),
                    h("div", {class: "mt-2"}, [
                        h(ShortcutSetting, {
                            label: "Hot-key",
                            value: hotKey ?? "Alt+space",
                            setValue: setHotKey,
                        }),
                    ]),
                ]),
            ]),

            h("div", {class: "w-full lg:w-1/2 mt-6"}, [
                h("h2", {class: "text-lg font-semibold mb-4 border-b pb-2"}, "Realtime Translation"),
                h("div", {class: "space-y-4"}, [
                    h("div", {class: "mt-2"}, [
                        h(Select, {
                            label: "Language",
                            value: realtimeTranslationLanguage ?? "en-US",
                            onChange: (e: KeyboardEvent) => setRealtimeTranslationLanguage((e.target as HTMLSelectElement).value),
                            options: [
                                {value: "en-US", label: "English (US)"},
                                {value: "ja-JP", label: "Japanese (JP)"},
                            ],
                        }),
                    ]),
                ]),
            ]),

            h("button", {
                onClick: async () => {
                    if (await confirm("Are you sure you want to reset to default settings?")) {
                        setAutoStart(false);
                        setAutoUpdate(false);
                        setHotKey("Alt+space");
                        setRealtimeTranslationLanguage(await locale() ?? "en-US");
                    }
                },
                class: "bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded mt-6",
            }, "Reset to default"),
        ])
    );
}

export default Settings;
