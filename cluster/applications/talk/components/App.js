import {h} from "https://cdn.skypack.dev/preact@10.22.1";
import {useEffect, useRef, useState} from "https://cdn.skypack.dev/preact@10.22.1/hooks";
import Sidebar from "./Sidebar.js";
import {HOST} from "../constants/host.js"
import {AudioHandler} from "../api/AudioHandler.js";
import useNotification from "../hooks/useNotification.js";

const SESSION_TIMEOUT_MS = 25 * 60 * 1000;
const RECONNECT_DELAY_MS = 1000;

const App = () => {
    const params = new URLSearchParams(window.location.search);

    const decodeConfig = (configString) => {
        try {
            const decoded = atob(configString);
            const bytes = new Uint8Array(decoded.split("").map(c => c.charCodeAt(0)));
            const parsed = JSON.parse(new TextDecoder().decode(bytes));

            return {
                outputMode: parsed.outputMode || parsed.o,
                voice: parsed.voice || parsed.v,
                instructions: parsed.instructions || parsed.i,
                temperature: parsed.temperature ?? parsed.t,
                prefixPaddingMs: parsed.prefixPaddingMs ?? parsed.p,
                silenceDurationMs: parsed.silenceDurationMs ?? parsed.s,
                threshold: parsed.threshold ?? parsed.th,
                transcriptionModel: parsed.transcriptionModel || parsed.tm,
            };
        } catch (error) {
            return null;
        }
    };

    let initialConfig = {};
    const compressedConfig = params.get("c");
    if (compressedConfig) {
        const decoded = decodeConfig(compressedConfig);
        if (decoded) {
            initialConfig = decoded;
        }
    } else {
        if (params.get("outputMode")) initialConfig.outputMode = params.get("outputMode");
        if (params.get("voice")) initialConfig.voice = params.get("voice");
        if (params.get("instructions")) initialConfig.instructions = params.get("instructions");
        if (params.get("temperature")) initialConfig.temperature = parseFloat(params.get("temperature"));
        if (params.get("prefixPaddingMs")) initialConfig.prefixPaddingMs = parseInt(params.get("prefixPaddingMs"), 10);
        if (params.get("silenceDurationMs")) initialConfig.silenceDurationMs = parseInt(params.get("silenceDurationMs"), 10);
        if (params.get("threshold")) initialConfig.threshold = parseFloat(params.get("threshold"));
        if (params.get("transcriptionModel")) initialConfig.transcriptionModel = params.get("transcriptionModel");
    }

    const [isConnected, setIsConnected] = useState(false);
    const [messages, setMessages] = useState([]);
    const [inputText, setInputText] = useState("");
    const [outputMode, setOutputMode] = useState(initialConfig.outputMode ?? "audio");
    const [isLoading, setIsLoading] = useState(false);
    const [showSidebar, setShowSidebar] = useState(false);
    const {notifications, showSuccess, showError} = useNotification();
    const audioHandlerRef = useRef(null);

    const currentUserMessageIdRef = useRef(null);
    const currentAssistantMessageIdRef = useRef(null);

    const [voice, setVoice] = useState(initialConfig.voice ?? "alloy");
    const [instructions, setInstructions] = useState(initialConfig.instructions ?? "You are a helpful assistant. Respond conversationally.");
    const [temperature, setTemperature] = useState(initialConfig.temperature ?? 0.8);
    const [prefixPaddingMs, setPrefixPaddingMs] = useState(initialConfig.prefixPaddingMs ?? 300);
    const [silenceDurationMs, setSilenceDurationMs] = useState(initialConfig.silenceDurationMs ?? 500);
    const [threshold, setThreshold] = useState(initialConfig.threshold ?? 0.5);
    const [transcriptionModel, setTranscriptionModel] = useState(initialConfig.transcriptionModel ?? "gpt-4o-transcribe");
    const [playbackRate, setPlaybackRate] = useState(1.0);

    useEffect(() => {
        if (audioHandlerRef.current) {
            audioHandlerRef.current.setPlaybackRate(playbackRate);
        }
    }, [playbackRate]);

    const wsRef = useRef(null);
    const localStreamRef = useRef(null);
    const messagesEndRef = useRef(null);
    const isConnectedRef = useRef(false);
    const isReconnectingRef = useRef(false);
    const messagesRef = useRef([]);

    const sessionTimerRef = useRef(null);
    const reconnectTimeoutRef = useRef(null);
    const [isReconnecting, setIsReconnecting] = useState(false);

    useEffect(() => {
        isConnectedRef.current = isConnected;
    }, [isConnected]);

    useEffect(() => {
        isReconnectingRef.current = isReconnecting;
    }, [isReconnecting]);

    useEffect(() => {
        messagesRef.current = messages;
    }, [messages]);

    const addMessage = (role, content, type = "text") => {
        const newMessage = {
            id: Date.now() + Math.random(),
            role,
            content,
            type,
            isStreaming: false
        };

        setMessages(previous => [...previous, newMessage]);
        return newMessage.id;
    };

    const createStreamingMessage = (role, type = "text") => {
        const newMessage = {
            id: Date.now() + Math.random(),
            role,
            content: "",
            type,
            isStreaming: true
        };

        setMessages(previous => [...previous, newMessage]);
        return newMessage.id;
    };

    const updateMessage = (messageId, updates) => {
        setMessages(previous => previous.map(message =>
            message.id === messageId ? {...message, ...updates} : message
        ));
    };

    const appendToMessage = (messageId, delta) => {
        setMessages(previous => previous.map(message =>
            message.id === messageId ? {...message, content: message.content + delta} : message
        ));
    };

    useEffect(() => {
        messagesEndRef.current?.scrollIntoView({behavior: "smooth"});
    }, [messages]);


    const startSessionTimer = () => {
        if (sessionTimerRef.current) {
            clearTimeout(sessionTimerRef.current);
        }

        sessionTimerRef.current = setTimeout(() => {
            handleSessionReconnect();
        }, SESSION_TIMEOUT_MS);
    };

    const clearSessionTimers = () => {
        if (sessionTimerRef.current) {
            clearTimeout(sessionTimerRef.current);
            sessionTimerRef.current = null;
        }
        if (reconnectTimeoutRef.current) {
            clearTimeout(reconnectTimeoutRef.current);
            reconnectTimeoutRef.current = null;
        }
    };

    const handleSessionReconnect = async () => {
        if (!isConnectedRef.current || isReconnectingRef.current) return;

        setIsReconnecting(true);

        showSuccess("Reconnecting session to maintain conversation...", 5000);

        if (wsRef.current) {
            wsRef.current.close();
        }

        reconnectTimeoutRef.current = setTimeout(async () => {
            await connectWebSocket(true);
            setIsReconnecting(false);
        }, RECONNECT_DELAY_MS);
    };

    const connectWebSocket = async (isReconnect = false) => {
        setIsLoading(true);

        const webSocketUrl = `wss://${HOST}/v1/realtime?model=gpt-realtime-mini-2025-12-15`;

        try {
            wsRef.current = new WebSocket(webSocketUrl, ["realtime", "openai-beta.realtime-v1"]);

            wsRef.current.onopen = () => {
                setIsConnected(true);
                setIsLoading(false);

                startSessionTimer();

                let sessionInstructions = instructions;

                if (isReconnect && messagesRef.current.length > 0) {
                    const conversationContext = messagesRef.current
                        .filter(message => !message.isStreaming && message.content)
                        .map(message => `${message.role}: ${message.content}`)
                        .join('\n');

                    if (conversationContext) {
                        sessionInstructions = `${instructions}\n\nPrevious conversation context:\n${conversationContext}\n\nPlease continue the conversation naturally, taking into account the previous context.`;
                    }
                }

                const sessionConfig = {
                    type: "session.update",
                    session: {
                        modalities: outputMode === "audio" ? ["text", "audio"] : ["text"],
                        instructions: sessionInstructions,
                        voice: voice,
                        input_audio_format: "pcm16",
                        output_audio_format: "pcm16",
                        input_audio_transcription: {
                            model: transcriptionModel
                        },
                        temperature: temperature,
                        turn_detection: {
                            type: "server_vad",
                            prefix_padding_ms: prefixPaddingMs,
                            silence_duration_ms: silenceDurationMs,
                            threshold: threshold,
                        }
                    }
                };

                wsRef.current.send(JSON.stringify(sessionConfig));

                if (isReconnect && messagesRef.current.length > 0) {
                    showSuccess("Session restored with conversation history", 3000);
                }
            };

            wsRef.current.onclose = () => {
                setIsConnected(false);
                setIsLoading(false);
                clearSessionTimers();
            };

            wsRef.current.onerror = () => {
                setIsConnected(false);
                setIsLoading(false);
                clearSessionTimers();

                if (!isReconnect) {
                    showError("Failed to connect to the server. Please check your API key and try again.");
                } else {
                    showError("Failed to reconnect session. Please try connecting manually.");
                }
            };

            try {
                localStreamRef.current = await navigator.mediaDevices.getUserMedia({
                    audio: {
                        echoCancellation: true,
                        noiseSuppression: true,
                        sampleRate: 24000
                    }
                });

                audioHandlerRef.current = new AudioHandler();
                await audioHandlerRef.current.initialize(localStreamRef.current);

                audioHandlerRef.current.onAudioData = (base64Audio) => {
                    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
                        wsRef.current.send(JSON.stringify({
                            type: "input_audio_buffer.append",
                            audio: base64Audio
                        }));
                    }
                };
            } catch (error) {
                showError("Microphone access is required for this application");
                disconnectWebSocket();
                return;
            }

            wsRef.current.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);

                    switch (data.type) {
                        case "session.created":
                        case "session.updated":
                            break;

                        case "input_audio_buffer.speech_started":
                            if (!currentUserMessageIdRef.current) {
                                currentUserMessageIdRef.current = createStreamingMessage("user", "audio");
                            }

                            if (currentAssistantMessageIdRef.current && wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
                                wsRef.current.send(JSON.stringify({
                                    type: "response.cancel"
                                }));
                            }

                            if (audioHandlerRef.current) {
                                audioHandlerRef.current.stopPlayback();
                            }
                            break;

                        case "input_audio_buffer.speech_stopped":
                            break;

                        case "input_audio_buffer.committed":
                            break;

                        case "conversation.item.input_audio_transcription.completed":
                            if (data.transcript && currentUserMessageIdRef.current) {
                                updateMessage(currentUserMessageIdRef.current, {
                                    content: data.transcript,
                                    isStreaming: false
                                });
                                currentUserMessageIdRef.current = null;
                            }
                            break;

                        case "conversation.item.input_audio_transcription.failed":
                            if (currentUserMessageIdRef.current) {
                                updateMessage(currentUserMessageIdRef.current, {
                                    content: "[Transcription failed]",
                                    isStreaming: false
                                });
                                currentUserMessageIdRef.current = null;
                            }
                            break;

                        case "conversation.item.input_audio_transcription.delta":
                            if (data.delta && currentUserMessageIdRef.current) {
                                appendToMessage(currentUserMessageIdRef.current, data.delta);
                            }
                            break;

                        case "input_audio_buffer.transcription":
                            if (data.transcript && currentUserMessageIdRef.current) {
                                updateMessage(currentUserMessageIdRef.current, {
                                    content: data.transcript
                                });
                            }
                            break;


                        case "response.created":
                        case "response.output_item.added":
                            if (!currentAssistantMessageIdRef.current) {
                                currentAssistantMessageIdRef.current = createStreamingMessage("assistant", outputMode);
                            }
                            break;

                        case "response.text.delta":
                            if (data.delta && currentAssistantMessageIdRef.current) {
                                appendToMessage(currentAssistantMessageIdRef.current, data.delta);
                            }
                            break;

                        case "response.text.done":
                            if (data.text && currentAssistantMessageIdRef.current) {
                                updateMessage(currentAssistantMessageIdRef.current, {
                                    content: data.text,
                                    isStreaming: false
                                });
                                currentAssistantMessageIdRef.current = null;
                            }
                            break;

                        case "response.audio_transcript.delta":
                            if (data.delta && currentAssistantMessageIdRef.current) {
                                appendToMessage(currentAssistantMessageIdRef.current, data.delta);
                            }
                            break;

                        case "response.audio_transcript.done":
                            if (data.transcript && currentAssistantMessageIdRef.current) {
                                updateMessage(currentAssistantMessageIdRef.current, {
                                    content: data.transcript,
                                    isStreaming: false
                                });
                                currentAssistantMessageIdRef.current = null;
                            }
                            break;

                        case "response.audio.delta":
                            if (data.delta && outputMode === "audio" && audioHandlerRef.current) {
                                audioHandlerRef.current.playAudioDelta(data.delta);
                            }
                            break;

                        case "response.done":
                            if (currentAssistantMessageIdRef.current) {
                                updateMessage(currentAssistantMessageIdRef.current, {
                                    isStreaming: false
                                });
                                currentAssistantMessageIdRef.current = null;
                            }
                            break;

                        case "error":
                            if (data.error) {
                                showError(`Error: ${data.error.message ?? "Unknown error"}`);
                            }
                            break;
                    }
                } catch (error) {
                    showError(`Failed to process message: ${error.message}`);
                }
            };

        } catch (error) {
            showError("Failed to establish WebSocket connection: " + error.message);
            setIsConnected(false);
            setIsLoading(false);
        }
    };

    const disconnectWebSocket = () => {
        if (audioHandlerRef.current) {
            audioHandlerRef.current.cleanup();
            audioHandlerRef.current = null;
        }

        if (wsRef.current) {
            wsRef.current.close();
            wsRef.current = null;
        }
        if (localStreamRef.current) {
            localStreamRef.current.getTracks().forEach(track => track.stop());
            localStreamRef.current = null;
        }

        clearSessionTimers();
        setIsConnected(false);
        setIsReconnecting(false);
    };

    const sendTextMessage = () => {
        if (!inputText.trim() || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;

        addMessage("user", inputText, "text");

        const message = {
            type: "conversation.item.create",
            item: {
                type: "message",
                role: "user",
                content: [{
                    type: "input_text",
                    text: inputText
                }]
            }
        };

        wsRef.current.send(JSON.stringify(message));

        wsRef.current.send(JSON.stringify({
            type: "response.create"
        }));

        setInputText("");
    };

    const updateOutputMode = (mode) => {
        setOutputMode(mode);

        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
            const sessionUpdate = {
                type: "session.update",
                session: {
                    modalities: mode === "audio" ? ["text", "audio"] : ["text"]
                }
            };
            wsRef.current.send(JSON.stringify(sessionUpdate));
        }
    };

    const shareConfig = () => {
        const config = {
            outputMode: outputMode,
            voice: voice,
            instructions: instructions,
            temperature: temperature,
            prefixPaddingMs: prefixPaddingMs,
            silenceDurationMs: silenceDurationMs,
            threshold: threshold,
            transcriptionModel: transcriptionModel,
        };

        const compressed = btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(config))));

        const shareUrl = `${window.location.origin}${window.location.pathname}?c=${encodeURIComponent(compressed)}`;

        navigator.clipboard.writeText(shareUrl).then(() => {
            showSuccess("Settings link copied to clipboard!", 3000);
        }).catch(error => {
            showError("Failed to copy link to clipboard");
        });
    };

    return h("div", {class: "flex h-screen bg-gray-100"}, [
        h(Sidebar, {
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
        }),

        notifications.success && h("div", {
            class: "fixed top-4 right-4 z-50 bg-green-500 text-white px-4 py-2 rounded-md shadow-lg animate-pulse"
        }, notifications.success),

        notifications.error && h("div", {
            class: "fixed top-4 right-4 z-50 bg-red-500 text-white px-4 py-2 rounded-md shadow-lg animate-pulse"
        }, notifications.error),

        h("div", {class: "flex flex-col flex-1"}, [
            h("div", {class: "bg-orange-600 text-white p-4 shadow-md"}, [
                h("div", {class: "max-w-4xl mx-auto"}, [

                    h("div", {class: "flex justify-between items-center"}, [
                        h("div", {class: "flex items-center gap-4"}, [
                            h("button", {
                                onClick: () => setShowSidebar(!showSidebar),
                                onPointerDown: (e) => e.stopPropagation(),
                                class: "text-white hover:text-gray-200 transition-colors",
                                title: "Settings"
                            }, h("svg", {
                                class: "w-6 h-6",
                                fill: "none",
                                stroke: "currentColor",
                                viewBox: "0 0 24 24"
                            }, [
                                h("path", {
                                    "stroke-linecap": "round",
                                    "stroke-linejoin": "round",
                                    "stroke-width": "2",
                                    d: "M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
                                })
                            ])),
                            h("h1", {class: "text-2xl font-bold"}, "Talk"),
                            isConnected && h("div", {
                                class: "flex items-center gap-2 ml-4",
                                title: "Microphone is active"
                            }, [
                                h("div", {class: "w-3 h-3 bg-green-400 rounded-full animate-pulse"}),
                                h("span", {class: "text-sm text-white/80"}, "Listening")
                            ])
                        ]),
                        h("div", {class: "flex gap-4 items-center"}, [
                            isReconnecting && h("div", {class: "flex items-center gap-2 mr-4"}, [
                                h("div", {class: "w-3 h-3 bg-yellow-400 rounded-full animate-pulse"}),
                                h("span", {class: "text-sm text-white/80"}, "Reconnecting...")
                            ]),
                            h("div", {class: "flex items-center gap-2"}, [
                                h("label", {class: "text-sm"}, "Output:"),
                                h("select", {
                                    value: outputMode,
                                    onChange: (event) => updateOutputMode(event.target.value),
                                    class: "bg-orange-700 text-white px-3 py-1 rounded-md focus:outline-none focus:ring-2 focus:ring-white",
                                }, [
                                    h("option", {value: "text"}, "Text"),
                                    h("option", {value: "audio"}, "Audio")
                                ])
                            ]),
                            isConnected ?
                                h("button", {
                                    onClick: disconnectWebSocket,
                                    class: "bg-red-500 hover:bg-red-600 px-4 py-2 rounded-md transition-colors"
                                }, "Disconnect") :
                                h("button", {
                                    onClick: connectWebSocket,
                                    disabled: isLoading,
                                    class: isLoading
                                        ? "bg-gray-400 px-4 py-2 rounded-md cursor-not-allowed"
                                        : "bg-green-500 hover:bg-green-600 px-4 py-2 rounded-md transition-colors"
                                }, isLoading ? "Loading..." : "Connect"),
                        ])
                    ])
                ])
            ]),

            h("div", {class: "flex-1 overflow-y-auto p-4"}, [
                h("div", {class: "max-w-4xl mx-auto space-y-4"}, [
                    ...messages.map((message) => {
                        return h("div", {
                            key: message.id,
                            class: `flex ${message.role === "user" ? "justify-end" : "justify-start"}`
                        }, [
                            h("div", {
                                class: `max-w-2xl p-4 rounded-lg ${
                                    message.role === "user"
                                        ? message.isStreaming ? "bg-orange-400 text-white animate-pulse" : "bg-orange-500 text-white"
                                        : message.isStreaming ? "bg-gray-200 text-gray-800" : "bg-white text-gray-800 shadow"
                                }`
                            }, [
                                h("div", {class: "text-sm opacity-75 mb-1"},
                                    message.isStreaming
                                        ? (message.role === "user" ? "Listening..." : "Assistant is typing...")
                                        : `${message.role === "user" ? "You" : "Assistant"} (${message.type})`
                                ),
                                message.content
                                    ? h("div", {class: "whitespace-pre-wrap"}, message.content)
                                    : message.isStreaming && h("div", {class: "flex items-center gap-2"}, [
                                    h("div", {class: "w-2 h-2 bg-current rounded-full animate-bounce"}),
                                    h("div", {
                                        class: "w-2 h-2 bg-current rounded-full animate-bounce",
                                        style: "animation-delay: 0.1s"
                                    }),
                                    h("div", {
                                        class: "w-2 h-2 bg-current rounded-full animate-bounce",
                                        style: "animation-delay: 0.2s"
                                    })
                                ])
                            ])
                        ]);
                    }),

                    h("div", {ref: messagesEndRef})
                ])
            ]),

            h("div", {class: "border-t bg-white p-4"}, [
                h("div", {class: "max-w-4xl mx-auto"}, [
                    h("div", {class: "flex gap-2"}, [
                        h("textarea", {
                            value: inputText,
                            onInput: (event) => setInputText(event.target.value),
                            onKeyPress: (event) => {
                                if (event.key === "Enter" && !event.shiftKey) {
                                    event.preventDefault();
                                    sendTextMessage();
                                }
                            },
                            placeholder: isConnected ? (isReconnecting ? "Reconnecting session..." : "Type a message...") : "Connect to start chatting",
                            disabled: !isConnected || isReconnecting,
                            class: "flex-1 p-3 border rounded-md resize-y focus:outline-none focus:ring-2 focus:ring-orange-500",
                            rows: 2
                        }),
                        h("button", {
                            onClick: sendTextMessage,
                            disabled: !isConnected || !inputText.trim() || isReconnecting,
                            class: `px-6 py-2 rounded-md transition-colors ${
                                isConnected && inputText.trim() && !isReconnecting
                                    ? "bg-orange-500 hover:bg-orange-600 text-white"
                                    : "bg-gray-300 text-gray-500 cursor-not-allowed"
                            }`
                        }, isReconnecting ? "Reconnecting..." : "Send")
                    ])
                ])
            ])
        ])
    ]);
};

export default App;
