import autofill from "./background/autofill.js";

const abortControllerMap = new Map();

const cortexReaderStreamMessageToDialogInTab = async (reader, tabId, abortController) => {
    const decoder = new TextDecoder();
    while (true) {
        if (abortController.signal.aborted) {
            break;
        }

        const {done, value} = await reader.read();
        if (done) {
            break;
        }
        const lines = decoder.decode(value).split("data: ").map((line) => line.trim()).filter((line) => line.length > 0);
        for (const line of lines) {
            if (line === "[DONE]") {
                break;
            }

            const json = JSON.parse(line);
            if (json.error !== undefined) {
                await chrome.tabs.sendMessage(tabId, {
                    type: "dialog:message",
                    content: json.error.message,
                });
                break;
            }
            if (json.choices?.length !== 1) {
                continue;
            }

            const choice = json.choices[0];

            switch (choice.finish_reason) {
                case "length": {
                    await chrome.tabs.sendMessage(tabId, {
                        type: "dialog:message",
                        content: "The maximum number of conversations has been exceeded.\nPlease create a new thread.",
                    });
                    return;
                }
                case "content_filter": {
                    await chrome.tabs.sendMessage(tabId, {
                        type: "dialog:message",
                        content: "Your message contains violent or explicit.",
                    });
                    return;
                }
                default:
                    break;
            }

            if (choice.delta.content === null) {
                continue;
            }

            await chrome.tabs.sendMessage(tabId, {
                type: "dialog:message",
                content: choice.delta.content,
            });
        }
    }
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
                    target: {tabId: tabs[0].id},
                    css: message.css,
                    files: message.files,
                });
            }
            case "translate": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Translating...",
                });

                const abortController = new AbortController();
                abortControllerMap["dialog:message"] = abortController;
                const response = await fetch("https://cortex-api.minikube.127.0.0.1.nip.io/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
                    body: JSON.stringify({
                        messages: [{
                            role: "system",
                            content: `Your task is to translate the following text into ${message.language || chrome.i18n.getUILanguage()}.`,
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
            case "summarize": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Summarizing...",
                });

                const abortController = new AbortController();
                abortControllerMap["dialog:message"] = abortController;
                const response = await fetch("https://cortex-api.minikube.127.0.0.1.nip.io/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
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
            case "respond": {
                const cookie = await cookieFromWebAuthFlow(tabs[0].id);

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Responding...",
                });

                const abortController = new AbortController();
                abortControllerMap["dialog:message"] = abortController;
                const response = await fetch("https://cortex-api.minikube.127.0.0.1.nip.io/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
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

                await chrome.tabs.sendMessage(tabs[0].id, {
                    type: "dialog:create",
                    content: "Suggesting...",
                });

                const abortController = new AbortController();
                abortControllerMap["dialog:message"] = abortController;
                const response = await fetch("https://cortex-api.minikube.127.0.0.1.nip.io/v1/chat/completions", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Cookie": cookie.value,
                    },
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
                abortControllerMap["dialog:message"] = abortController;

                return autofill(message, cookie, tabs[0].id, abortController);
            }
        }
    });
}

chrome.runtime.onMessage.addListener(handleMessage);

const cookieFromWebAuthFlow = (tabId) => {
    return chrome.identity.launchWebAuthFlow({
        url: `https://bakery.minikube.127.0.0.1.nip.io/callback?cookie_name=_oauth2_proxy&redirect_url=${encodeURIComponent(chrome.identity.getRedirectURL("extension"))}`,
    }).then((responseURL) => {
        const url = new URL(responseURL);
        const value = url.searchParams.get("value");
        const expires = url.searchParams.get("expires");
        return Promise.resolve({value, expires});
    }).catch((error) => {
        if (error.toString().startsWith("Error: User interaction required.")) {
            return chrome.tabs.create({
                url: `https://bakery.minikube.127.0.0.1.nip.io/callback?cookie_name=_oauth2_proxy&redirect_url=${encodeURIComponent(chrome.identity.getRedirectURL("extension"))}`,
            }).then((tab) => {
                return new Promise((resolve) => {
                    let i = 0;
                    chrome.tabs.onUpdated.addListener(function listener(id, info) {
                        if (info.status === "complete" && id === tab.id) {
                            i++;
                            if (i === 2) { // HACK: Wait to redirect
                                chrome.tabs.onUpdated.removeListener(listener);
                                resolve(tab);
                            }
                        }
                    });
                });
            }).then((tab) => {
                return chrome.tabs.remove(tab.id);
            }).then(() => {
                return chrome.tabs.update(tabId, {highlighted: true});
            }).then(() => {
                return cookieFromWebAuthFlow(tabId);
            });
        } else {
            return chrome.tabs.reload(tabId).then(() => {
                return new Promise((resolve) => {
                    chrome.tabs.onUpdated.addListener(function listener(id, info) {
                        if (info.status === "complete" && id === id) {
                            chrome.tabs.onUpdated.removeListener(listener);
                            resolve();
                        }
                    });
                });
            }).then(() => {
                return cookieFromWebAuthFlow(tabId);
            });
        }
    });
}

chrome.contextMenus.onClicked.addListener((message) => {
    return handleMessage({
        type: message.menuItemId,
        content: message.selectionText,
    });
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
});
