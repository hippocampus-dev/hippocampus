(() => {
    const interval = 1000;
    const selector = `div>div>div>div>div>div>div>span:not(:has(*)):not([class])`;

    const cache = new Map();
    const logsMap = new Map();

    const logLoop = async () => {
        const now = Date.now();

        [...logsMap.keys()].sort().forEach((index) => {
            const entry = logsMap.get(index);

            if (now - entry.deletedAt > interval) {
                console.log(`${entry.name}: ${entry.text}`);
                logsMap.delete(index);
            }
        });

        await new Promise((resolve) => setTimeout(resolve, interval));
        await logLoop();
    };

    logLoop().catch(console.error);

    let i = 0;
    new MutationObserver((records) => {
        records.forEach((record) => {
            switch (record.type) {
                case "childList": {
                    record.addedNodes.forEach((addedNode) => {
                        if (addedNode.nodeType === Node.ELEMENT_NODE) {
                            [...document.querySelectorAll(selector)].forEach((node) => {
                                const name = node?.parentNode?.parentNode?.previousSibling?.lastChild?.textContent;
                                const text = node?.textContent?.trim();

                                if (name === undefined || text === undefined || text === "") {
                                    return;
                                }

                                const entry = {name, text, createdAt: Date.now(), index: ++i};
                                cache.set(node, entry);
                            });
                        }
                    });

                    record.removedNodes.forEach((removedNode) => {
                        const entry = cache.get(removedNode);
                        if (entry === undefined) {
                            return;
                        }

                        entry.deletedAt = Date.now();

                        logsMap.set(entry.index, entry);
                        cache.delete(removedNode);
                    });

                    break;
                }
                case "characterData": {
                    const entry = cache.get(record.target.parentNode);
                    if (entry === undefined) {
                        return;
                    }

                    const text = record.target.data.trim();
                    if (text === "") {
                        return;
                    }

                    entry.text = text;
                    entry.updatedAt = Date.now();

                    break;
                }
            }
        });
    }).observe(document.body, {
        childList: true,
        characterData: true,
        subtree: true,
    });
})();
