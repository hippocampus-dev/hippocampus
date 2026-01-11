(() => {
    const selector = `ytd-watch-next-secondary-results-renderer`;

    let blockedChannels = [];

    const loadSettings = () => {
        chrome.storage.sync.get(['blockedChannels'], function (items) {
            blockedChannels = items.blockedChannels || [];
        });
    };

    loadSettings();

    chrome.storage.onChanged.addListener((changes, namespace) => {
        if (namespace === 'sync' && changes.blockedChannels) {
            loadSettings();
        }
    });

    const block = (nodes) => {
        [...nodes].forEach((node) => {
            const channelElement = node.querySelector(`yt-content-metadata-view-model > div.yt-content-metadata-view-model__metadata-row > span`);
            if (!channelElement) {
                return;
            }

            const channelName = channelElement.textContent;

            if (blockedChannels.includes(channelName)) {
                node.style.display = "none";
                return;
            }
        });
    };

    block(document.querySelectorAll(selector));

    new MutationObserver((records) => {
        records.forEach((record) => {
            record.addedNodes.forEach((addedNode) => {
                if (addedNode.nodeType === Node.ELEMENT_NODE) {
                    if (addedNode.matches && addedNode.matches(selector)) {
                        block([addedNode]);
                    } else {
                        block(addedNode.querySelectorAll(selector));
                    }
                }
            });
        });
    }).observe(document.body, {
        childList: true,
        subtree: true,
    });
})();
