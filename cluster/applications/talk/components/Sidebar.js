import {h} from "https://cdn.skypack.dev/preact@10.22.1";
import {useEffect, useRef} from "https://cdn.skypack.dev/preact@10.22.1/hooks";

const Sidebar = ({
                     showSidebar,
                     setShowSidebar,
                     voice,
                     setVoice,
                     instructions,
                     setInstructions,
                     temperature,
                     setTemperature,
                     prefixPaddingMs,
                     setPrefixPaddingMs,
                     silenceDurationMs,
                     setSilenceDurationMs,
                     threshold,
                     setThreshold,
                     transcriptionModel,
                     setTranscriptionModel,
                     playbackRate,
                     setPlaybackRate,
                     shareConfig,
                     isConnected
                 }) => {
    const sidebarRef = useRef(null);

    useEffect(() => {
        const handleClickOutside = (event) => {
            if (showSidebar && sidebarRef.current && !sidebarRef.current.contains(event.target)) {
                setShowSidebar(false);
            }
        };

        document.addEventListener("pointerdown", handleClickOutside);
        return () => {
            document.removeEventListener("pointerdown", handleClickOutside);
        };
    }, [showSidebar, setShowSidebar]);

    return h("div", {
        ref: sidebarRef,
        class: `fixed inset-y-0 left-0 z-50 w-80 bg-white shadow-lg transform transition-transform duration-300 ${showSidebar ? 'translate-x-0' : '-translate-x-full'}`
    }, [
        h("div", {class: "p-6 h-full overflow-y-auto"}, [
            h("div", {class: "flex justify-between items-center mb-6"}, [
                h("h2", {class: "text-xl font-bold text-gray-800"}, "Settings"),
                h("button", {
                    onClick: () => setShowSidebar(false),
                    class: "text-gray-500 hover:text-gray-700"
                }, "✕")
            ]),

            h("div", {class: "mb-6"}, [
                h("label", {class: "block text-sm font-medium text-gray-700 mb-2"}, "Voice"),
                h("select", {
                    value: voice,
                    onChange: (event) => setVoice(event.target.value),
                    disabled: isConnected,
                    class: "w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-orange-500 " + (isConnected ? "bg-gray-100 cursor-not-allowed" : "")
                }, [
                    h("option", {value: "alloy"}, "Alloy"),
                    h("option", {value: "ash"}, "Ash"),
                    h("option", {value: "ballad"}, "Ballad"),
                    h("option", {value: "coral"}, "Coral"),
                    h("option", {value: "echo"}, "Echo"),
                    h("option", {value: "fable"}, "Fable"),
                    h("option", {value: "onyx"}, "Onyx"),
                    h("option", {value: "nova"}, "Nova"),
                    h("option", {value: "sage"}, "Sage"),
                    h("option", {value: "shimmer"}, "Shimmer"),
                    h("option", {value: "verse"}, "Verse")
                ])
            ]),

            h("div", {class: "mb-6"}, [
                h("label", {class: "block text-sm font-medium text-gray-700 mb-2"}, "Transcription Model"),
                h("select", {
                    value: transcriptionModel,
                    onChange: (event) => setTranscriptionModel(event.target.value),
                    disabled: isConnected,
                    class: "w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-orange-500 " + (isConnected ? "bg-gray-100 cursor-not-allowed" : "")
                }, [
                    h("option", {value: "gpt-4o-transcribe"}, "GPT-4o Transcribe"),
                    h("option", {value: "gpt-4o-mini-transcribe"}, "GPT-4o Mini Transcribe"),
                    h("option", {value: "whisper-1"}, "Whisper-1")
                ])
            ]),

            h("div", {class: "mb-6"}, [
                h("label", {class: "block text-sm font-medium text-gray-700 mb-2"}, "Instructions"),
                h("textarea", {
                    value: instructions,
                    onInput: (event) => setInstructions(event.target.value),
                    disabled: isConnected,
                    rows: 4,
                    class: "w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-orange-500 resize-y" + (isConnected ? "bg-gray-100 cursor-not-allowed" : "")
                })
            ]),

            h("div", {class: "mb-6"}, [
                h("label", {class: "block text-sm font-medium text-gray-700 mb-2"}, `Temperature: ${temperature}`),
                h("input", {
                    type: "range",
                    min: "0",
                    max: "2",
                    step: "0.1",
                    value: temperature,
                    onInput: (event) => setTemperature(parseFloat(event.target.value)),
                    disabled: isConnected,
                    class: "w-full " + (isConnected ? "cursor-not-allowed opacity-50" : "")
                })
            ]),

            h("div", {class: "mb-6"}, [
                h("label", {class: "block text-sm font-medium text-gray-700 mb-2"}, `Playback Speed: ${playbackRate}x`),
                h("input", {
                    type: "range",
                    min: "0.5",
                    max: "2.0",
                    step: "0.1",
                    value: playbackRate,
                    onInput: (event) => setPlaybackRate(parseFloat(event.target.value)),
                    class: "w-full"
                })
            ]),

            h("div", {class: "mb-4"}, [
                h("h3", {class: "text-sm font-semibold text-gray-700 mb-3"}, "Turn Detection Settings")
            ]),

            h("div", {class: "mb-4"}, [
                h("label", {class: "block text-sm font-medium text-gray-700 mb-2"}, `Prefix Padding: ${prefixPaddingMs}ms`),
                h("input", {
                    type: "range",
                    min: "0",
                    max: "1000",
                    step: "50",
                    value: prefixPaddingMs,
                    onInput: (event) => setPrefixPaddingMs(parseInt(event.target.value, 10)),
                    disabled: isConnected,
                    class: "w-full " + (isConnected ? "cursor-not-allowed opacity-50" : "")
                })
            ]),

            h("div", {class: "mb-4"}, [
                h("label", {class: "block text-sm font-medium text-gray-700 mb-2"}, `Silence Duration: ${silenceDurationMs}ms`),
                h("input", {
                    type: "range",
                    min: "100",
                    max: "2000",
                    step: "100",
                    value: silenceDurationMs,
                    onInput: (event) => setSilenceDurationMs(parseInt(event.target.value, 10)),
                    disabled: isConnected,
                    class: "w-full " + (isConnected ? "cursor-not-allowed opacity-50" : "")
                })
            ]),

            h("div", {class: "mb-6"}, [
                h("label", {class: "block text-sm font-medium text-gray-700 mb-2"}, `Threshold: ${threshold}`),
                h("input", {
                    type: "range",
                    min: "0",
                    max: "1",
                    step: "0.1",
                    value: threshold,
                    onInput: (event) => setThreshold(parseFloat(event.target.value)),
                    disabled: isConnected,
                    class: "w-full " + (isConnected ? "cursor-not-allowed opacity-50" : "")
                })
            ]),

            h("div", {class: "mb-6"}, [
                h("button", {
                    onClick: shareConfig,
                    class: "w-full px-4 py-2 bg-orange-500 hover:bg-orange-600 text-white rounded-md transition-colors flex items-center justify-center gap-2"
                }, [
                    h("svg", {
                        class: "w-5 h-5",
                        fill: "none",
                        stroke: "currentColor",
                        viewBox: "0 0 24 24"
                    }, [
                        h("path", {
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            d: "M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m9.632 4.316C18.114 15.438 18 15.982 18 16.5c0 1.381-1.119 2.5-2.5 2.5S13 17.881 13 16.5s1.119-2.5 2.5-2.5c.518 0 1.062.114 1.316.316m0 0l2.184 2.184m0 0L21 18.5m-2-2l2-2M12 9a3 3 0 100-6 3 3 0 000 6z"
                        })
                    ]),
                    "Share Settings"
                ])
            ]),

            isConnected && h("div", {class: "text-sm text-gray-500 italic"}, [
                "Settings are locked during active connection. Disconnect to make changes."
            ])
        ])
    ]);
};

export default Sidebar;
