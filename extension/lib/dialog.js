/* Example:
const show = (message) => {
    const selection = window.getSelection();
    if (selection === null || selection.toString().length === 0) {
        return;
    }

    for (const element of document.getElementsByTagName("extension-dialog-root")) {
        element.remove();
    }

    const range = selection.getRangeAt(0);
    const rect = range.getBoundingClientRect();

    const dialog = createDialog(rect, "Translating...");
    document.body.after(dialog);

    dialog.dispatchEvent(new CustomEvent("message", {
        detail: message.selection,
    }));
}
*/

const markdownLinkRegexp = /\[([^\]]+)\]\(([^)]+)\)/g;
const markdownify = (text) => {
    return text.replace(markdownLinkRegexp, `<a href="$2">$1</a>`).replace(/\n/g, "<br>");
}
const markdownUnLinkRegexp = /<a href="([^"]+)">([^<]+)<\/a>/g;
const unMarkdownify = (text) => {
    return text.replace(markdownUnLinkRegexp, "[$2]($1)").replace(/<br>/g, "\n");
}

class DialogOptsBuilder {
    constructor() {
        this.ephemeral = true;
        this.original = null;
    }

    withEphemeral(ephemeral) {
        this.ephemeral = ephemeral;
        return this;
    }

    withOriginal(original) {
        this.original = original;
        return this;
    }

    build() {
        return {
            ephemeral: this.ephemeral,
            original: this.original,
        };
    }
}

// Chrome Extension does not support Custom Elements
const createDialog = (rect, title, opts = new DialogOptsBuilder().build()) => {
    const root = document.createElement("extension-dialog-root");
    root.style.position = "absolute";
    root.style.top = "0px";
    root.style.left = "0px";
    root.style.width = "100%";
    root.style.zIndex = "2147483647";
    root.style.setProperty("--dialog-width", "600px");

    const dialog = document.createElement("div");
    dialog.style.position = "absolute";
    dialog.style.left = rect.left;
    dialog.style.top = rect.bottom;
    dialog.setAttribute("role", "dialog");
    root.appendChild(dialog);

    const box = document.createElement("div");
    box.style.display = "flex";
    box.style.flexDirection = "column";
    box.style.padding = "8px";
    box.style.width = "var(--dialog-width)";
    box.style.maxHeight = "80vh";
    box.style.backgroundColor = "white";
    box.style.borderRadius = "4px";
    box.style.boxShadow = "0px 0px 10px rgba(0, 0, 0, 0.4)"
    dialog.appendChild(box);

    const flex = document.createElement("div");
    flex.style.display = "flex";
    flex.style.flexDirection = "row";
    flex.style.alignItems = "center";
    flex.style.paddingBottom = "4px";
    flex.style.gap = "4px";
    box.appendChild(flex);

    const avatar = document.createElement("img");
    avatar.style.width = "16px";
    avatar.style.height = "16px";
    avatar.style.verticalAlign = "top";
    avatar.src = chrome.runtime.getURL("images/icon32.png");
    avatar.alt = "Extension";
    flex.appendChild(avatar);

    const titleBlock = document.createElement("div");
    titleBlock.style.fontSize = "16px";
    titleBlock.style.color = "black";
    titleBlock.innerText = title;
    flex.appendChild(titleBlock);

    const select = document.createElement("select");
    select.style.padding = "4px";
    select.style.marginLeft = "auto";
    select.style.color = "black";
    select.style.backgroundColor = "white";
    select.style.border = "1px solid #e0e0e0";
    select.style.borderRadius = "4px";
    select.style.cursor = "pointer";
    flex.appendChild(select);

    [
        "-",
        "en-US",
        "ja-JP",
    ].forEach((language) => {
        const option = document.createElement("option");
        option.value = language;
        option.innerText = language;
        select.appendChild(option);
    });

    select.addEventListener("change", () => {
        const language = select.value;
        chrome.runtime.sendMessage({
            type: "translate",
            content: textBlock.innerText,
            language: language,
        });
    });

    if (opts.original) {
        const reroll = document.createElement("button");
        reroll.style.padding = "0px";
        reroll.style.width = "16px";
        reroll.style.height = "16px";
        reroll.style.backgroundColor = "transparent";
        reroll.style.border = "none";
        reroll.style.cursor = "pointer";
        reroll.setAttribute("aria-label", "Reroll");
        buttons.appendChild(reroll);

        reroll.addEventListener("pointerover", () => {
            reroll.style.opacity = "0.5";
        });

        reroll.addEventListener("pointerout", () => {
            reroll.style.opacity = "1";
        });

        reroll.addEventListener("pointerdown", () => {
            reroll.style.opacity = "0.8";
        });

        reroll.addEventListener("click", () => {
            chrome.runtime.sendMessage(opts.original);
        });

        const rerollIcon = document.createElement("img");
        rerollIcon.style.width = "16px";
        rerollIcon.style.height = "16px";
        rerollIcon.style.verticalAlign = "top";
        rerollIcon.src = chrome.runtime.getURL("images/reroll.svg");
        rerollIcon.alt = "Reroll";
        reroll.appendChild(rerollIcon);
    }

    const clipboardTooltip = document.createElement("div");
    clipboardTooltip.style.display = "none";
    clipboardTooltip.style.position = "absolute";
    clipboardTooltip.style.right = "0px";
    clipboardTooltip.style.bottom = "100%";
    clipboardTooltip.style.padding = "4px";
    clipboardTooltip.style.color = "white";
    clipboardTooltip.style.backgroundColor = "black";
    clipboardTooltip.style.borderRadius = "4px";
    clipboardTooltip.style.zIndex = "2147483647";
    clipboardTooltip.innerText = "Copied!";
    flex.appendChild(clipboardTooltip);

    const buttons = document.createElement("div");
    buttons.style.display = "flex";
    buttons.style.flexDirection = "row";
    buttons.style.alignItems = "center";
    buttons.style.paddingTop = "4px";
    buttons.style.gap = "4px";
    buttons.style.marginLeft = "auto";
    flex.appendChild(buttons);

    const clipboard = document.createElement("button");
    clipboard.style.padding = "0px";
    clipboard.style.width = "16px";
    clipboard.style.height = "16px";
    clipboard.style.backgroundColor = "transparent";
    clipboard.style.border = "none";
    clipboard.style.cursor = "pointer";
    clipboard.setAttribute("aria-label", "Copy to clipboard");
    buttons.appendChild(clipboard);

    clipboard.addEventListener("pointerover", () => {
        clipboard.style.opacity = "0.5";
    });

    clipboard.addEventListener("pointerout", () => {
        clipboard.style.opacity = "1";
    });

    clipboard.addEventListener("pointerdown", () => {
        clipboard.style.opacity = "0.8";
    });

    clipboard.addEventListener("click", () => {
        navigator.clipboard.writeText(unMarkdownify(textBlock.innerHTML));

        clipboardTooltip.style.display = "block";
        setTimeout(() => {
            clipboardTooltip.style.display = "none";
        }, 1000);
    });

    const clipboardIcon = document.createElement("img");
    clipboardIcon.style.width = "16px";
    clipboardIcon.style.height = "16px";
    clipboardIcon.style.verticalAlign = "top";
    clipboardIcon.src = chrome.runtime.getURL("images/clipboard.svg");
    clipboardIcon.alt = "Copy to clipboard";
    clipboard.appendChild(clipboardIcon);

    const close = document.createElement("button");
    close.style.padding = "0px";
    close.style.width = "16px";
    close.style.height = "16px";
    close.style.backgroundColor = "transparent";
    close.style.border = "none";
    close.style.cursor = "pointer";
    close.setAttribute("aria-label", "Close");
    buttons.appendChild(close);

    close.addEventListener("pointerover", () => {
        close.style.opacity = "0.5";
    });

    close.addEventListener("pointerout", () => {
        close.style.opacity = "1";
    });

    close.addEventListener("pointerdown", () => {
        close.style.opacity = "0.8";
    });

    close.addEventListener("click", () => {
        root.remove();
    });

    const closeIcon = document.createElement("img");
    closeIcon.style.width = "16px";
    closeIcon.style.height = "16px";
    closeIcon.style.verticalAlign = "top";
    closeIcon.src = chrome.runtime.getURL("images/close.svg");
    closeIcon.alt = "Close";
    close.appendChild(closeIcon);

    const divider = document.createElement("div");
    divider.style.borderTop = "1px solid #e0e0e0";
    box.appendChild(divider);

    const stack = document.createElement("div");
    stack.style.paddingTop = "4px";
    stack.style.textAlign = "left";
    stack.style.overflowY = "auto";
    stack.style.flexGrow = "1";
    box.appendChild(stack);

    const textBlock = document.createElement("div");
    textBlock.style.fontSize = "14px";
    textBlock.style.color = "black";
    textBlock.style.overflowWrap = "break-word";
    stack.appendChild(textBlock);

    root.addEventListener("message", (event) => {
        textBlock.innerText += event.detail;

        setTimeout(() => {
            textBlock.innerHTML = markdownify(textBlock.innerHTML);
        }, 1000);
    });

    if (opts.ephemeral) {
        document.addEventListener("pointerdown", function clickOutside(event) {
            if (root.contains(event.target)) {
                return;
            }

            root.remove();
            document.removeEventListener("pointerdown", clickOutside);
        });

        document.addEventListener("keydown", function keydown(event) {
            if (event.key !== "Escape") {
                return;
            }

            root.remove();
            document.removeEventListener("keydown", keydown);
        });
    }

    return root;
}
