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

        const menu = createMenu(rect, [
            {
                label: "Translate",
                callback: (self) => {
                    self.remove();

                    const selection = window.getSelection();
                    chrome.runtime.sendMessage({
                        type: "translate",
                        content: selection.toString(),
                    });
                },
            },
        ]);

        document.body.after(menu);
    });
    document.body.after(tooltip);
});
*/

class MenuOptsBuilder {
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

const createMenu = (rect, callbacks, opts = new MenuOptsBuilder().build()) => {
    const DEFAULT_BACKGROUND_COLOR = "#ffffff";

    const root = document.createElement("extension-menu-root");
    root.style.position = "absolute";
    root.style.top = "0px";
    root.style.left = "0px";
    root.style.width = "100%";
    root.style.zIndex = "2147483646";

    const menu = document.createElement("div");
    menu.style.position = "absolute";
    menu.style.left = rect.left;
    menu.style.top = rect.bottom;
    menu.style.padding = "4px";
    menu.style.borderRadius = "4px";
    menu.style.backgroundColor = DEFAULT_BACKGROUND_COLOR;
    menu.style.boxShadow = "0px 0px 10px rgba(0, 0, 0, 0.4)"
    menu.style.minWidth = "150px";
    menu.setAttribute("role", "menu");
    root.appendChild(menu);

    callbacks.forEach(({label, callback}) => {
        const item = document.createElement("div");
        item.textContent = label;
        item.style.padding = "8px";
        item.style.cursor = "pointer";
        item.style.userSelect = "none";

        item.addEventListener("pointerover", () => {
            item.style.backgroundColor = "#f0f0f0";
        });

        item.addEventListener("pointerout", () => {
            item.style.backgroundColor = DEFAULT_BACKGROUND_COLOR;
        });

        item.addEventListener("pointerdown", (event) => {
            item.style.backgroundColor = "#e0e0e0";
        });

        item.addEventListener("click", () => {
            callback(root);
        });

        menu.appendChild(item);
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
};
