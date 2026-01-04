(() => {
    const selector = `[aria-label="広告"]`;

    const hideAds = (nodes) => {
        [...nodes].forEach((node) => {
            node.style.display = "none";
        });
    };

    hideAds(document.querySelectorAll(selector));

    new MutationObserver((records) => {
        records.forEach((record) => {
            record.addedNodes.forEach((addedNode) => {
                if (addedNode.nodeType === Node.ELEMENT_NODE) {
                    hideAds(addedNode.querySelectorAll(selector));
                }
            });
        });
    }).observe(document.body, {
        childList: true,
        subtree: true,
    });
})();
