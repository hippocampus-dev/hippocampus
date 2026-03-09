(() => {
    const monitor = (selector, cb) => {
        cb(document.querySelectorAll(selector));

        new MutationObserver((records) => {
            records.forEach((record) => {
                if (record.type === "attributes") {
                    if (record.target.nodeType === Node.ELEMENT_NODE) {
                        cb(record.target.querySelectorAll(selector));
                    }
                }
                record.addedNodes.forEach((addedNode) => {
                    if (addedNode.nodeType === Node.ELEMENT_NODE) {
                        cb(addedNode.querySelectorAll(selector));
                    }
                });
            });
        }).observe(document.body, {
            childList: true,
            subtree: true,
            attributes: true,
            attributeFilter: ["role"],
        });
    };

    monitor(`[role="textbox"]`, (nodes) => {
        const showTooltip = (node) => {
            return () => {
                for (const element of document.getElementsByTagName("extension-tooltip-root")) {
                    element.remove();
                }

                const rect = {
                    left: "calc(var(--tooltip-size) * -1)",
                    bottom: "0px",
                };

                const optsBuilder = new TooltipOptsBuilder().withEphemeral(false);
                const tooltip = createTooltip(rect, (_self) => {
                    const items = document.querySelectorAll(`[role="listitem"]`);
                    if (items.length === 0) {
                        const content = node.textContent;
                        chrome.runtime.sendMessage({
                            type: "email",
                            content: content,
                        });
                    } else {
                        const content = [...items].map((item) => item.textContent).join("\n");
                        chrome.runtime.sendMessage({
                            type: "respond",
                            content: content,
                        });
                    }
                }, optsBuilder.build());

                document.querySelectorAll(".tooltip-anchor").forEach((element) => element.classList.remove("tooltip-anchor"));
                node.classList.add("tooltip-anchor");

                tooltip.style.positionAnchor = "--tooltip";
                tooltip.style.positionArea = "bottom right";

                document.body.after(tooltip);
            }
        };
        [...nodes]
            .forEach((node) => {
                node.removeEventListener("focus", showTooltip(node));
                node.addEventListener("focus", showTooltip(node));
            });
    });
})();
