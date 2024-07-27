(() => {
    const URL_PATTERN = new RegExp("(/[^/]+/[^/]+/blob/[^/]+/).+");
    const FILE_PATH_PATTERN = new RegExp("(.+\.yaml)");

    const monitor = (selector, cb) => {
        cb(document.querySelectorAll(selector));

        new MutationObserver((records) => {
            records.forEach((record) => {
                record.addedNodes.forEach((addedNode) => {
                    if (addedNode.nodeType === Node.ELEMENT_NODE) {
                        cb(addedNode.querySelectorAll(selector));
                    }
                });
            });
        }).observe(document.body, {
            childList: true,
            subtree: true,
        });
    };

    monitor("[data-code-text]", (nodes) => {
        const textarea = document.getElementById("read-only-cursor-text-area");
        if (textarea === null) {
            return;
        }
        textarea.style.zIndex = "0";

        const urlMatches = location.pathname.match(URL_PATTERN);

        [...nodes]
            .filter((node) => FILE_PATH_PATTERN.test(node?.dataset?.codeText) && node?.classList?.contains("pl-s"))
            .forEach((node) => {
                const filePathMatches = node.dataset.codeText.match(FILE_PATH_PATTERN);
                const anchor = document.createElement("a");
                anchor.href = `${location.origin}/${urlMatches[1]}${filePathMatches[1]}`;
                Object.assign(anchor.style, {
                    position: "absolute",
                    inset: "0",
                    pointerEvents: "auto",
                });
                Object.assign(node.style, {
                    textDecoration: "underline",
                });
                node.appendChild(anchor);
            });
    });
})();
