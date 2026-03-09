import {cookieFromWebAuthFlow} from "./lib/cookie.js";
import {parseSSEStream} from "./lib/sse.js";

const markdownLinkRegexp = /\[([^\]]+)\]\(([^)]+)\)/g;
const markdownify = (text) => {
    return text.replace(markdownLinkRegexp, `<a href="$2" target="_blank">$1</a>`).replace(/\n/g, "<br>");
};

const messagesContainer = document.getElementById("messages");
const inputElement = document.getElementById("input");
const sendButton = document.getElementById("send");
const contextChips = document.getElementById("context-chips");
const modelSelect = document.getElementById("model");
const newConversationButton = document.getElementById("new-conversation");

let messages = [];
let abortController = null;
let currentSelectionText = "";
let suppressedSelection = null;

const scrollToBottom = () => {
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
};

const createCopyButton = (text) => {
    const button = document.createElement("button");
    button.className = "copy-button";
    button.title = "Copy";

    const icon = document.createElement("img");
    icon.src = "images/clipboard.svg";
    icon.alt = "Copy";
    button.appendChild(icon);

    button.addEventListener("click", () => {
        navigator.clipboard.writeText(text);
    });

    return button;
};

const appendMessage = (role, content, context) => {
    const element = document.createElement("div");
    element.className = `message ${role}`;
    element.innerHTML = markdownify(content);

    if (context) {
        const contextElement = document.createElement("div");
        contextElement.className = "context";
        contextElement.textContent = context.length > 200 ? context.slice(0, 200) + "..." : context;
        element.appendChild(contextElement);
    }

    if (role === "assistant") {
        element.appendChild(createCopyButton(content));
    }

    messagesContainer.appendChild(element);
    scrollToBottom();
    return element;
};

const createStreamingMessage = () => {
    const element = document.createElement("div");
    element.className = "message assistant";
    messagesContainer.appendChild(element);
    scrollToBottom();
    return element;
};

const setStreaming = (streaming) => {
    if (streaming) {
        sendButton.textContent = "Stop";
        sendButton.classList.add("stop");
        inputElement.disabled = true;
    } else {
        sendButton.textContent = "Send";
        sendButton.classList.remove("stop");
        inputElement.disabled = false;
        inputElement.focus();
    }
};

const send = async () => {
    const text = inputElement.value.trim();
    if (text.length === 0) {
        return;
    }

    inputElement.value = "";
    inputElement.style.height = "auto";

    const selectionChip = contextChips.querySelector(".context-chip");
    const selectionText = selectionChip ? currentSelectionText : "";

    const userContent = selectionText.length > 0
        ? `${text}\n\n---\n\n${selectionText}`
        : text;

    appendMessage("user", text, selectionText.length > 0 ? selectionText : null);
    messages.push({role: "user", content: userContent});

    setStreaming(true);
    abortController = new AbortController();

    const streamingElement = createStreamingMessage();
    let assistantContent = "";

    try {
        const tabs = await chrome.tabs.query({active: true, currentWindow: true});
        const cookie = await cookieFromWebAuthFlow(tabs[0]?.id);

        const response = await fetch("https://cortex-api.kaidotio.dev/v1/chat/completions", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Cookie": cookie.value,
            },
            redirect: "manual",
            body: JSON.stringify({
                model: modelSelect.value,
                messages: messages,
                stream: true,
            }),
            signal: abortController.signal,
        });

        const reader = response.body?.getReader();
        if (!reader) {
            return;
        }

        for await (const event of parseSSEStream(reader, abortController)) {
            switch (event.type) {
                case "content":
                    assistantContent += event.content;
                    streamingElement.innerHTML = markdownify(assistantContent);
                    scrollToBottom();
                    break;
                case "error":
                    assistantContent += event.message;
                    streamingElement.innerHTML = markdownify(assistantContent);
                    scrollToBottom();
                    break;
                case "finish":
                    if (event.reason === "length") {
                        assistantContent += "\n\n(The maximum number of conversations has been exceeded.)";
                    } else if (event.reason === "content_filter") {
                        assistantContent += "\n\n(Your message contains violent or explicit content.)";
                    }
                    streamingElement.innerHTML = markdownify(assistantContent);
                    scrollToBottom();
                    break;
            }
        }
    } catch (error) {
        if (error.name !== "AbortError") {
            assistantContent += `\n\n(Error: ${error.message})`;
            streamingElement.innerHTML = markdownify(assistantContent);
            scrollToBottom();
        }
    }

    if (assistantContent.length > 0) {
        messages.push({role: "assistant", content: assistantContent});
        streamingElement.appendChild(createCopyButton(assistantContent));
    }

    abortController = null;
    setStreaming(false);
};

sendButton.addEventListener("click", () => {
    if (abortController !== null) {
        abortController.abort();
        return;
    }
    send();
});

inputElement.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        if (abortController !== null) {
            return;
        }
        send();
    }
});

inputElement.addEventListener("input", () => {
    inputElement.style.height = "auto";
    inputElement.style.height = Math.min(inputElement.scrollHeight, 120) + "px";
});

const getSelectionText = async () => {
    const tabs = await chrome.tabs.query({active: true, currentWindow: true});
    if (tabs.length === 0) {
        return "";
    }
    const results = await chrome.scripting.executeScript({
        target: {tabId: tabs[0].id},
        func: () => window.getSelection()?.toString() ?? "",
    });
    return results[0]?.result ?? "";
};

const updateSelectionChip = (text) => {
    const existing = contextChips.querySelector(".context-chip");
    if (text.length === 0) {
        if (existing) {
            existing.remove();
        }
        return;
    }
    const preview = text.slice(0, 30) + (text.length > 30 ? "..." : "");
    if (existing) {
        existing.querySelector(".context-chip-preview").textContent = preview;
        return;
    }
    const chip = document.createElement("div");
    chip.className = "context-chip";

    const label = document.createElement("span");
    label.className = "context-chip-label";
    label.textContent = "Selection";
    chip.appendChild(label);

    const previewElement = document.createElement("span");
    previewElement.className = "context-chip-preview";
    previewElement.textContent = preview;
    chip.appendChild(previewElement);

    const close = document.createElement("button");
    close.title = "Remove";
    const icon = document.createElement("img");
    icon.src = "images/close.svg";
    icon.alt = "Remove";
    close.appendChild(icon);
    close.addEventListener("click", () => {
        suppressedSelection = currentSelectionText || null;
        chip.remove();
    });
    chip.appendChild(close);

    contextChips.appendChild(chip);
};

const pollSelection = async () => {
    if (!document.hidden) {
        try {
            currentSelectionText = await getSelectionText();
        } catch {
            currentSelectionText = "";
        }
        if (currentSelectionText.length > 0 && currentSelectionText !== suppressedSelection) {
            suppressedSelection = null;
            updateSelectionChip(currentSelectionText);
        } else if (currentSelectionText.length === 0) {
            suppressedSelection = null;
            updateSelectionChip("");
        }
    }
    setTimeout(pollSelection, 1000);
};
pollSelection();

newConversationButton.addEventListener("click", () => {
    messages = [];
    messagesContainer.innerHTML = "";
    if (abortController !== null) {
        abortController.abort();
        abortController = null;
        setStreaming(false);
    }
});
