(() => {
    const URL_PATTERN = new RegExp("(/[^/]+/[^/]+/blob/[^/]+/).+");
    const FILE_PATH_PATTERN = new RegExp("(.+\.yaml)");

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

    monitor("div.AppHeader-localBar", (nodes) => {
        [...nodes].forEach((node) => {
            const navigations = node.querySelectorAll("div.AppHeader-localBar>nav>ul>li");
            if (navigations.length === 0) {
                return;
            }

            const codeNavigation = navigations[0];

            const appendNavigation = (identifier, filepath) => {
                const newNavigation = codeNavigation.cloneNode(true);

                const anchor = newNavigation.querySelector("a#code-tab");
                anchor.id = `${identifier}-tab`;
                anchor.href = anchor.href + `/blob/HEAD/${filepath}`;

                const content = newNavigation.querySelector(`span[data-content="Code"]`);
                content.textContent = identifier;

                if (document.getElementById(anchor.id) === null) {
                    codeNavigation.parentNode.appendChild(newNavigation);
                }
            };

            appendNavigation("Workflows", ".github/workflows")
        });
    });
})();
