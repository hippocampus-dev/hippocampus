import {Link} from "preact-router";
import {h} from "preact";
import {useState, useEffect, useRef} from "preact/hooks";
import {getCurrentWindow, LogicalSize} from "@tauri-apps/api/window";
import {getVersion} from "@tauri-apps/api/app";
import {confirm} from "@tauri-apps/plugin-dialog";
import {invoke, Channel} from "@tauri-apps/api/core";
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
    const [voiceInputShortcut, setVoiceInputShortcut] = useState<string | null>(null);
    const [voiceInputModel, setVoiceInputModel] = useState<string | null>(null);
    const [voiceInputLanguage, setVoiceInputLanguage] = useState<string | null>(null);
    const [voiceInputDevice, setVoiceInputDevice] = useState<string | null>(null);
    const [modelDownloaded, setModelDownloaded] = useState(false);
    const [downloading, setDownloading] = useState(false);
    const [downloadProgress, setDownloadProgress] = useState(0);
    const [audioDevices, setAudioDevices] = useState<string[]>([]);

    const ref = useRef<HTMLDivElement | null>(null);

    useEffect(() => {
        (async () => {
            const [settings, devices] = await Promise.all([
                invoke<IPCSettings>("get_settings"),
                invoke<string[]>("list_audio_input_devices").catch((e) => {
                    console.error("Failed to list audio devices:", e);
                    return [] as string[];
                }),
            ]);

            setAudioDevices(devices);

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
                    case "handle_voice_input_shortcut":
                        setVoiceInputShortcut(setting.current_value);
                        break;
                    case "handle_voice_input_model":
                        setVoiceInputModel(setting.current_value);
                        break;
                    case "handle_voice_input_language":
                        setVoiceInputLanguage(setting.current_value);
                        break;
                    case "handle_voice_input_device":
                        setVoiceInputDevice(setting.current_value);
                        break;
                }
            });
        })();
    }, []);

    useEffect(() => {
        if (voiceInputModel === null) return;
        (async () => {
            try {
                const status: boolean = await invoke("get_whisper_model_status", {modelName: voiceInputModel});
                setModelDownloaded(status);
            } catch (e) {
                console.error("Failed to check model status:", e);
            }
        })();
    }, [voiceInputModel]);

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

    useEffect(() => {
        if (voiceInputShortcut === null) return;
        (async () => {
            await invoke("handle_voice_input_shortcut", {value: voiceInputShortcut});
        })();
    }, [voiceInputShortcut]);

    useEffect(() => {
        if (voiceInputModel === null) return;
        (async () => {
            await invoke("handle_voice_input_model", {value: voiceInputModel});
        })();
    }, [voiceInputModel]);

    useEffect(() => {
        if (voiceInputLanguage === null) return;
        (async () => {
            await invoke("handle_voice_input_language", {value: voiceInputLanguage});
        })();
    }, [voiceInputLanguage]);

    useEffect(() => {
        if (voiceInputDevice === null) return;
        (async () => {
            await invoke("handle_voice_input_device", {value: voiceInputDevice});
        })();
    }, [voiceInputDevice]);

    const handleDownloadModel = async () => {
        if (!voiceInputModel || downloading) return;
        setDownloading(true);
        setDownloadProgress(0);
        try {
            const channel = new Channel<number>();
            channel.onmessage = (progress: number) => {
                setDownloadProgress(progress);
            };
            await invoke("download_whisper_model", {modelName: voiceInputModel, progressTx: channel});
            setModelDownloaded(true);
        } catch (e) {
            console.error("Failed to download model:", e);
        } finally {
            setDownloading(false);
        }
    };

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

            h("div", {class: "w-full lg:w-1/2 mt-6"}, [
                h("h2", {class: "text-lg font-semibold mb-4 border-b pb-2"}, "Voice Input"),
                h("div", {class: "space-y-4"}, [
                    h("div", {class: "mt-2"}, [
                        h(ShortcutSetting, {
                            label: "Shortcut",
                            value: voiceInputShortcut ?? "Alt+Shift+space",
                            setValue: setVoiceInputShortcut,
                        }),
                    ]),
                    h("div", {class: "mt-2"}, [
                        h(Select, {
                            label: "Model",
                            value: voiceInputModel ?? "base",
                            onChange: (e: KeyboardEvent) => setVoiceInputModel((e.target as HTMLSelectElement).value),
                            options: [
                                {value: "tiny", label: "Tiny (75 MB)"},
                                {value: "tiny.en", label: "Tiny English (75 MB)"},
                                {value: "base", label: "Base (142 MB)"},
                                {value: "base.en", label: "Base English (142 MB)"},
                                {value: "small", label: "Small (466 MB)"},
                                {value: "small.en", label: "Small English (466 MB)"},
                                {value: "medium", label: "Medium (1.5 GB)"},
                                {value: "medium.en", label: "Medium English (1.5 GB)"},
                                {value: "large-v1", label: "Large v1 (2.9 GB)"},
                                {value: "large-v2", label: "Large v2 (2.9 GB)"},
                                {value: "large-v3", label: "Large v3 (2.9 GB)"},
                                {value: "large-v3-turbo", label: "Large v3 Turbo (1.5 GB)"},
                            ],
                        }),
                    ]),
                    h("div", {class: "mt-2"}, [
                        h(Select, {
                            label: "Language",
                            value: voiceInputLanguage ?? "auto",
                            onChange: (e: KeyboardEvent) => setVoiceInputLanguage((e.target as HTMLSelectElement).value),
                            options: [
                                {value: "auto", label: "Auto"},
                                {value: "en", label: "English"},
                                {value: "ja", label: "Japanese"},
                                {value: "zh", label: "Chinese"},
                                {value: "ko", label: "Korean"},
                                {value: "de", label: "German"},
                                {value: "fr", label: "French"},
                                {value: "es", label: "Spanish"},
                            ],
                        }),
                    ]),
                    h("div", {class: "mt-2"}, [
                        h(Select, {
                            label: "Device",
                            value: voiceInputDevice ?? "Default",
                            onChange: (e: KeyboardEvent) => setVoiceInputDevice((e.target as HTMLSelectElement).value),
                            options: [
                                {value: "Default", label: "Default"},
                                ...audioDevices.map(d => ({value: d, label: d})),
                            ],
                        }),
                    ]),
                    h("div", {class: "mt-2 flex items-center space-x-4"}, [
                        modelDownloaded
                            ? h("span", {class: "text-green-600 font-medium"}, "Model downloaded")
                            : downloading
                                ? h("div", {class: "flex-1"}, [
                                    h("div", {class: "w-full bg-gray-200 rounded-full h-4"}, [
                                        h("div", {
                                            class: "bg-blue-600 h-4 rounded-full transition-all",
                                            style: {width: `${Math.round(downloadProgress * 100)}%`},
                                        }),
                                    ]),
                                    h("span", {class: "text-sm text-gray-600 mt-1"}, `${Math.round(downloadProgress * 100)}%`),
                                ])
                                : h("button", {
                                    onClick: handleDownloadModel,
                                    class: "bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded",
                                }, "Download Model"),
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
                        setVoiceInputShortcut("Alt+Shift+space");
                        setVoiceInputModel("base");
                        setVoiceInputLanguage("auto");
                        setVoiceInputDevice("Default");
                    }
                },
                class: "bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded mt-6",
            }, "Reset to default"),
        ])
    );
}

export default Settings;
