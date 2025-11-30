/* Example:
document.addEventListener("selectionchange", () => {
    const selection = window.getSelection();
    if (selection === undefined || selection.toString().length === 0) {
        return;
    }

    for (const element of document.getElementsByTagName("extension-tooltip-root")) {
        element.remove();
    }

    const range = selection.getRangeAt(0);
    const rect = range.getBoundingClientRect();

    const tooltip = createTooltip(rect, (self) => {
        self.remove();

        chrome.runtime.sendMessage({
            type: "translate",
            content: selection.toString(),
        });
    });
    document.body.after(tooltip);
});
*/

class TooltipOptsBuilder {
    constructor() {
        this.ephemeral = true;
    }

    withEphemeral(ephemeral) {
        this.ephemeral = ephemeral;
        return this;
    }

    build() {
        return {
            ephemeral: this.ephemeral,
        };
    }
}

// Chrome Extension does not support Custom Elements
const createTooltip = (rect, callback, opts = new TooltipOptsBuilder().build()) => {
    const DEFAULT_BACKGROUND_COLOR = "#ffffff";

    const root = document.createElement("extension-tooltip-root");
    root.style.position = "absolute";
    root.style.top = "0px";
    root.style.left = "0px";
    root.style.width = "100%";
    root.style.zIndex = "2147483646";
    root.style.setProperty("--tooltip-size", "16px");

    const tooltip = document.createElement("div");
    tooltip.style.position = "absolute";
    tooltip.style.left = rect.left;
    tooltip.style.top = rect.bottom;
    tooltip.setAttribute("role", "tooltip");
    root.appendChild(tooltip);

    const box = document.createElement("div");
    box.style.display = "flex";
    box.style.flexDirection = "row";
    box.style.alignItems = "center";
    box.style.padding = "2px";
    box.style.backgroundColor = DEFAULT_BACKGROUND_COLOR;
    box.style.borderRadius = "var(--tooltip-size)";
    box.style.boxShadow = "0px 0px 10px rgba(0, 0, 0, 0.4)"
    tooltip.appendChild(box);

    const button = document.createElement("button");
    button.style.padding = "0px";
    button.style.width = "var(--tooltip-size)";
    button.style.height = "var(--tooltip-size)";
    button.style.backgroundColor = "transparent";
    button.style.border = "none";
    button.style.cursor = "pointer";
    button.setAttribute("aria-label", "Tooltip");
    box.appendChild(button);

    const avatar = document.createElement("img");
    avatar.style.width = "var(--tooltip-size)";
    avatar.style.height = "var(--tooltip-size)";
    avatar.style.verticalAlign = "top";
    avatar.src = chrome.runtime.getURL("images/icon32.png");
    avatar.alt = "Tooltip";
    button.appendChild(avatar);

    button.addEventListener("pointerover", () => {
        box.style.backgroundColor = "#f0f0f0";
    });

    button.addEventListener("pointerout", () => {
        box.style.backgroundColor = DEFAULT_BACKGROUND_COLOR;
    });

    button.addEventListener("pointerdown", () => {
        box.style.backgroundColor = "#e0e0e0";
    });

    button.addEventListener("click", () => {
        callback(root);
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
