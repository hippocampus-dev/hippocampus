const autofill = async (message, cookie, tabId, abortController) => {
    await chrome.tabs.sendMessage(tabId, {
        type: "dialog:create",
        content: "Autofilling...",
    });

    const systemPrompt = (entry) => {
        switch (entry.type) {
            case "Multiple choice":
                return `
Your task is to select one of the following options according to the user input.
MUST respond the selected option exactly without any modifications.

Available options:
${entry.options.map((option) => `- ${option}`).join("\n")}
`;
            case "Checkboxes":
            case "Dropdown":
                return `
Your task is to select one or more of the following options according to the user input.
MUST respond comma-separated the selected options exactly without any modifications.

Available options:
${entry.options.map((option) => `- ${option}`).join("\n")}
`;
            default:
                return `Your task is to fill ${entry.type} entry according to the user input in ${chrome.i18n.getUILanguage()}.`;
        }
    };

    const tasks = message.content.entries.map(async (entry) => {
        const response = await fetch("https://cortex-api.kaidotio.dev/v1/chat/completions", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Cookie": cookie.value,
            },
            body: JSON.stringify({
                messages: [{
                    role: "system",
                    content: systemPrompt(entry),
                }, {
                    role: "user",
                    content: entry.name,
                }],
            }),
            signal: abortController.signal,
        });
        const json = await response.json();
        if (json.error !== undefined) {
            throw new Error(json.error);
        }
        if (json.choices?.length !== 1) {
            return;
        }

        const choice = json.choices[0];

        const result = choice.message.content;
        await chrome.tabs.sendMessage(tabId, {
            type: "dialog:message",
            content: `${entry.name}: ${result}\n\n`,
        });
        switch (entry.type) {
            case "Multiple choice":
            case "Checkboxes":
            case "Dropdown":
                return result.split(",").map((option) => option.trim()).map((option) => {
                    return `entry.${entry.id}=${encodeURIComponent(option)}`;
                });
            default:
                return `entry.${entry.id}=${encodeURIComponent(result)}`;
        }
    });
    const prefill = await Promise.all(tasks).then((results) => {
        return results.flat();
    }).catch(async (error) => {
        await chrome.tabs.sendMessage(tabId, {
            type: "dialog:message",
            content: error,
        });
        return [];
    });
    if (prefill.length > 0) {
        const url = `${message.content.url}?${prefill.join("&")}`.replace("/formResponse", "/viewform");
        return chrome.tabs.sendMessage(tabId, {
            type: "dialog:message",
            content: `[Autofilled URL](${url})`,
        });
    }
};

export default autofill;
