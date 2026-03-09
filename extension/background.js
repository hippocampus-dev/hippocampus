import autofill from "./background/autofill.js";
import {cookieFromWebAuthFlow} from "./lib/cookie.js";
import {parseSSEStream} from "./lib/sse.js";

fetch("https://kaidotio-hippocampus.pages.dev/extension/manifest.json").then((response) => {
    return response.json();
}).then((json) => {
    if (chrome.runtime.getManifest().version !== json.version) {
        chrome.action.setBadgeBackgroundColor({color: "red"});
        chrome.action.setBadgeText({text: "Old"});
    }
});

const abortControllerMap = new Map();

const cortexReaderStreamMessageToDialogInTab = async (reader, tabId, abortController) => {
    for await (const event of parseSSEStream(reader, abortController)) {
        switch (event.type) {
            case "content":
                await chrome.tabs.sendMessage(tabId, {
                    type: "dialog:message",
                    content: event.content,
                });
                break;
            case "error":
                await chrome.tabs.sendMessage(tabId, {
                    type: "dialog:message",
                    content: event.message,
                });
                break;
            case "finish":
                if (event.reason === "length") {
                    await chrome.tabs.sendMessage(tabId, {
                        type: "dialog:message",
                        content: "The maximum number of conversations has been exceeded.",
                    });
                } else if (event.reason === "content_filter") {
                    await chrome.tabs.sendMessage(tabId, {
                        type: "dialog:message",
                        content: "Your message contains violent or explicit.",
                    });
                }
                return;
        }
    }
}

const cortexReaderStreamMessageToDOMInTab = async (reader, tabId, id, abortController) => {
    for await (const event of parseSSEStream(reader, abortController)) {
        switch (event.type) {
            case "content":
                await chrome.tabs.sendMessage(tabId, {
                    type: "dom:update",
                    id: id,
                    text: event.content,
                });
                break;
            case "error":
                await chrome.tabs.sendMessage(tabId, {
                    type: "dom:error",
                    error: event.message,
                });
                return;
            case "finish":
                if (event.reason === "length") {
                    await chrome.tabs.sendMessage(tabId, {
                        type: "dom:error",
                        error: "The maximum number of conversations has been exceeded.",
                    });
                } else if (event.reason === "content_filter") {
                    await chrome.tabs.sendMessage(tabId, {
                        type: "dom:error",
                        error: "Your message contains violent or explicit content.",
                    });
                }
                return;
        }
    }

    await chrome.tabs.sendMessage(tabId, {
        type: "dom:done",
        id: id,
    });
}

const handleMessage = (message) => {
    return chrome.tabs.query({
        active: true,
        currentWindow: true
    }).then(async (tabs) => {
        switch (message.type) {
            case "abort": {
                const abortController = abortControllerMap[message.content];
                if (abortController !== undefined && !abortController.signal.aborted) {
                    abortController.abort();
                }
                return;
            }
            case "styler": {
                return chrome.scripting.insertCSS({
                    target: {tabId: tabs[0].id, allFrames: true},
                    css: message.css,
                    files: message.files,
                });
            }
            case "translate": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                const abortController = new AbortController();
                if (abortControllerMap["dialog:message"] !== undefined && !abortControllerMap["dialog:message"].signal.aborted) {
                    abortControllerMap["dialog:message"].abort();
                }
                abortControllerMap["dialog:message"] = abortController;

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Translating...",
                });

                const response = await fetch("https://cortex-api.kaidotio.dev/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
                    redirect: "manual",
                    body: JSON.stringify({
                        messages: [{
                            role: "system",
                            content: `
Translate the given text according to the following rules.
- **Return proper nouns, URLs, code snippets, blockquotes, and any content already in ${message.language ?? chrome.i18n.getUILanguage()} without translation.**
- **NEVER change the document structure for all languages, including line breaks, URLs, code snippets, blockquotes, etc**.
- Translate into ${message.language ?? chrome.i18n.getUILanguage()}.`,
                        }, {
                            role: "user",
                            content: message.content,
                        }],
                        stream: true,
                    }),
                    signal: abortController.signal,
                });
                const reader = await response.body?.getReader();
                if (reader === null) {
                    return;
                }
                return cortexReaderStreamMessageToDialogInTab(reader, tabs[0].id, abortController);
            }
            case "translate-dom": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                const abortController = new AbortController();
                if (abortControllerMap["dom:update"] !== undefined) {
                    abortControllerMap["dom:update"].abort();
                }
                abortControllerMap["dom:update"] = abortController;

                const MAX_CONCURRENT_TRANSLATIONS = 10;

                const processNode = async (node) => {
                    if (abortController.signal.aborted) {
                        return;
                    }

                    const response = await fetch("https://cortex-api.kaidotio.dev/v1/chat/completions", {
                        method: "POST",
                        headers: {
                            "Content-Type": "application/json",
                            "Cookie": cookie.value,
                        },
                        body: JSON.stringify({
                            messages: [{
                                role: "system",
                                content: `
Translate the given text according to the following rules.
- **Return proper nouns, URLs, code snippets, blockquotes, and any content already in ${message.language ?? chrome.i18n.getUILanguage()} without translation.**
- **NEVER change the document structure for all languages, including line breaks, URLs, code snippets, blockquotes, etc**.
- Translate into ${message.language ?? chrome.i18n.getUILanguage()}.`,
                            }, {
                                role: "user",
                                content: node.text,
                            }],
                            stream: true,
                        }),
                        signal: abortController.signal,
                    });
                    const reader = await response.body?.getReader();
                    if (reader === null) {
                        return;
                    }
                    await cortexReaderStreamMessageToDOMInTab(reader, tabs[0].id, node.id, abortController);
                };

                const queue = [...message.nodes];
                const inProgress = new Set();

                while (queue.length > 0 || inProgress.size > 0) {
                    if (abortController.signal.aborted) {
                        break;
                    }

                    while (queue.length > 0 && inProgress.size < MAX_CONCURRENT_TRANSLATIONS) {
                        const node = queue.shift();
                        const promise = processNode(node).then(() => {
                            inProgress.delete(promise);
                        });
                        inProgress.add(promise);
                    }

                    if (inProgress.size > 0) {
                        await Promise.race(inProgress);
                    }
                }

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dom:complete"
                });

                return;
            }
            case "summarize": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                const abortController = new AbortController();
                if (abortControllerMap["dialog:message"] !== undefined && !abortControllerMap["dialog:message"].signal.aborted) {
                    abortControllerMap["dialog:message"].abort();
                }
                abortControllerMap["dialog:message"] = abortController;

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Summarizing...",
                });

                const response = await fetch("https://cortex-api.kaidotio.dev/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
                    redirect: "manual",
                    body: JSON.stringify({
                        messages: [{
                            role: "system",
                            content: `Your task is to create a concise running summary of actions and information results in the provided text in ${chrome.i18n.getUILanguage()}, focusing on key and potentially important information to remember.`,
                        }, {
                            role: "user",
                            content: message.content,
                        }],
                        stream: true,
                    }),
                    signal: abortController.signal,
                });
                const reader = await response.body?.getReader();
                if (reader === null) {
                    return;
                }
                return cortexReaderStreamMessageToDialogInTab(reader, tabs[0].id, abortController);
            }
            case "email": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                const abortController = new AbortController();
                if (abortControllerMap["dialog:message"] !== undefined && !abortControllerMap["dialog:message"].signal.aborted) {
                    abortControllerMap["dialog:message"].abort();
                }
                abortControllerMap["dialog:message"] = abortController;

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Email creating...",
                    original: message,
                });

                const response = await fetch("https://cortex-api.kaidotio.dev/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
                    redirect: "manual",
                    body: JSON.stringify({
                        messages: [{
                            role: "system",
                            content: `Your task is to create an email body from the following text in ${chrome.i18n.getUILanguage()}.`,
                        }, {
                            role: "user",
                            content: message.content,
                        }],
                        stream: true,
                    }),
                    signal: abortController.signal,
                });
                const reader = await response.body?.getReader();
                if (reader === null) {
                    return;
                }
                return cortexReaderStreamMessageToDialogInTab(reader, tabs[0].id, abortController);
            }
            case "respond": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                const abortController = new AbortController();
                if (abortControllerMap["dialog:message"] !== undefined && !abortControllerMap["dialog:message"].signal.aborted) {
                    abortControllerMap["dialog:message"].abort();
                }
                abortControllerMap["dialog:message"] = abortController;

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Responding...",
                });

                const response = await fetch("https://cortex-api.kaidotio.dev/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
                    redirect: "manual",
                    body: JSON.stringify({
                        messages: [{
                            role: "system",
                            content: `Your task is to create an email response to the following text in ${chrome.i18n.getUILanguage()}.`,
                        }, {
                            role: "user",
                            content: message.content,
                        }],
                        stream: true,
                    }),
                    signal: abortController.signal,
                });
                const reader = await response.body?.getReader();
                if (reader === null) {
                    return;
                }
                return cortexReaderStreamMessageToDialogInTab(reader, tabs[0].id, abortController);
            }
            case "suggest": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                const abortController = new AbortController();
                if (abortControllerMap["dialog:message"] !== undefined && !abortControllerMap["dialog:message"].signal.aborted) {
                    abortControllerMap["dialog:message"].abort();
                }
                abortControllerMap["dialog:message"] = abortController;

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Suggesting...",
                });

                const response = await fetch("https://cortex-api.kaidotio.dev/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
                    redirect: "manual",
                    body: JSON.stringify({
                        messages: [{
                            role: "system",
                            content: `Your task is to suggest a resolution to the following text in ${chrome.i18n.getUILanguage()}.`,
                        }, {
                            role: "user",
                            content: message.content,
                        }],
                        stream: true,
                    }),
                    signal: abortController.signal,
                });
                const reader = await response.body?.getReader();
                if (reader === null) {
                    return;
                }
                return cortexReaderStreamMessageToDialogInTab(reader, tabs[0].id, abortController);
            }
            case "autofill": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                const abortController = new AbortController();
                if (abortControllerMap["dialog:message"] !== undefined && !abortControllerMap["dialog:message"].signal.aborted) {
                    abortControllerMap["dialog:message"].abort();
                }
                abortControllerMap["dialog:message"] = abortController;

                return autofill(message, cookie, tabs[0].id, abortController);
            }
        }
    });
}

chrome.runtime.onMessage.addListener(handleMessage);

chrome.contextMenus.onClicked.addListener(async (message, tab) => {
    if (message.menuItemId.startsWith("translate-dom:")) {
        const language = message.menuItemId.replace("translate-dom:", "");
        await chrome.scripting.executeScript({
            target: {tabId: tab.id},
            func: (language) => {
                if (typeof domProcessor === "function") {
                    const p = domProcessor((nodes) => {
                        chrome.runtime.sendMessage({
                            type: "translate-dom",
                            nodes: nodes,
                            language: language,
                        });
                    });
                    p(document.body);
                }
            },
            args: [language],
        });
    } else if (message.menuItemId.startsWith("translate-dom-with-original:")) {
        const language = message.menuItemId.replace("translate-dom-with-original:", "");
        await chrome.scripting.executeScript({
            target: {tabId: tab.id},
            func: (language) => {
                if (typeof domProcessor === "function") {
                    const p = domProcessor((nodes) => {
                        chrome.runtime.sendMessage({
                            type: "translate-dom",
                            nodes: nodes,
                            language: language,
                        });
                    }, {keepOriginal: true});
                    p(document.body);
                }
            },
            args: [language],
        });
    } else if (message.menuItemId === "chat") {
        chrome.sidePanel.open({tabId: tab.id});
    } else {
        return handleMessage({
            type: message.menuItemId,
            content: message.selectionText,
        });
    }
});

const parentTabMap = new Map();

const matchUrlPattern = (url, pattern) => {
    const escapeRegExp = (string) => string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

    let regexPattern = pattern;
    regexPattern = escapeRegExp(regexPattern);
    regexPattern = regexPattern.replace(/\\\*/g, '.*');
    regexPattern = '^' + regexPattern + '$';

    try {
        const regex = new RegExp(regexPattern);
        return regex.test(url);
    } catch {
        return false;
    }
};

chrome.tabs.onCreated.addListener((tab) => {
    if (!tab.openerTabId) return;

    chrome.storage.sync.get(["autoCloseUrlPatterns"], (items) => {
        const urlPatterns = items.autoCloseUrlPatterns || [];

        if (urlPatterns.length === 0) {
            return;
        }

        parentTabMap.set(tab.id, tab.openerTabId);

        chrome.tabs.get(tab.openerTabId, (parentTab) => {
            let shouldAutoClose = false;

            for (const pattern of urlPatterns) {
                if (matchUrlPattern(parentTab.url, pattern)) {
                    shouldAutoClose = true;
                    break;
                }
            }

            if (shouldAutoClose && parentTab.url !== tab.pendingUrl && parentTab.url !== tab.url) {
                setTimeout(() => {
                    chrome.tabs.get(tab.id, (currentTab) => {
                        if (currentTab && !currentTab.active) {
                            chrome.tabs.remove(tab.id);
                        }
                    });
                }, 10);
            }
        });
    });
});

chrome.tabs.onRemoved.addListener((tabId) => {
    parentTabMap.delete(tabId);
});

chrome.runtime.onInstalled.addListener(() => {
    chrome.contextMenus.create({
        id: "translate",
        title: "Translate",
        contexts: ["selection"],
    });
    chrome.contextMenus.create({
        id: "summarize",
        title: "Summarize",
        contexts: ["selection"],
    });
    chrome.contextMenus.create({
        id: "chat",
        title: "Chat",
    });
    chrome.contextMenus.create({
        id: "translate-dom",
        title: "Translate DOM",
    });
    [
        {id: "ja", name: "Japanese"},
        {id: "en", name: "English"},
    ].forEach(language => {
        chrome.contextMenus.create({
            parentId: "translate-dom",
            id: `translate-dom:${language.id}`,
            title: language.name,
        });
    });
    chrome.contextMenus.create({
        id: "translate-dom-with-original",
        title: "Translate DOM with Original",
    });
    [
        {id: "ja", name: "Japanese"},
        {id: "en", name: "English"},
    ].forEach(language => {
        chrome.contextMenus.create({
            parentId: "translate-dom-with-original",
            id: `translate-dom-with-original:${language.id}`,
            title: language.name,
        });
    });
});
