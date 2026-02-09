const domProcessor = (callback, options = {}) => {
    const {keepOriginal = false} = options;
    const PROCESSING_CLASS = "processing-anchor";
    const textNodeMap = new Map();
    const translationBuffers = new Map();

    const collectTextNodes = (rootElement, textNodeMap) => {
        const walker = document.createTreeWalker(
            rootElement,
            NodeFilter.SHOW_TEXT,
            {
                acceptNode: (node) => {
                    const parent = node.parentElement;

                    if (node.textContent.trim().length === 0) {
                        return NodeFilter.FILTER_REJECT;
                    }

                    if (!parent) {
                        return NodeFilter.FILTER_REJECT;
                    }

                    const excludedTags = [
                        "script",
                        "style",
                        "code",
                        "pre",
                        "kbd",
                        "var",
                        "samp",
                        "textarea",
                        "input",
                        "select",
                        "svg",
                        "math",
                        "iframe",
                        "object",
                        "embed",
                        "applet",
                    ];

                    if (parent.matches(excludedTags.join(", "))) {
                        return NodeFilter.FILTER_REJECT;
                    }

                    if (parent.closest('[translate="no"]')) {
                        return NodeFilter.FILTER_REJECT;
                    }

                    const excludedClasses = [
                        PROCESSING_CLASS,
                        "notranslate",
                        "code",
                        "highlight",
                        "hljs",
                    ];

                    const hasExcludedClass = excludedClasses.some(className =>
                        parent.classList.contains(className) ||
                        parent.closest(`.${className}`)
                    );

                    if (hasExcludedClass) {
                        return NodeFilter.FILTER_REJECT;
                    }

                    if (parent.isContentEditable || parent.closest('[contenteditable="true"]')) {
                        return NodeFilter.FILTER_REJECT;
                    }

                    if (parent.hasAttribute("aria-label") && !parent.hasAttribute("aria-labelledby")) {
                        return NodeFilter.FILTER_REJECT;
                    }

                    if (parent.matches('[href^="javascript:"], [href^="data:"]')) {
                        return NodeFilter.FILTER_REJECT;
                    }

                    return NodeFilter.FILTER_ACCEPT;
                }
            }
        );

        const textNodes = [];
        let index = 0;
        while (walker.nextNode()) {
            const node = walker.currentNode;
            textNodeMap.set(index, node);
            textNodes.push({id: index, text: node.textContent});
            index++;
        }
        return textNodes;
    };

    const addTranslatingEffect = (node) => {
        const parent = node.parentElement;
        if (parent && !parent.classList.contains(PROCESSING_CLASS)) {
            parent.classList.add(PROCESSING_CLASS);
            parent.style.opacity = "0.6";
            parent.style.transition = "opacity 0.3s ease";
        }
    };

    const removeTranslatingEffect = (node) => {
        const parent = node.parentElement;
        if (parent && parent.classList.contains(PROCESSING_CLASS)) {
            parent.classList.remove(PROCESSING_CLASS);
            parent.style.opacity = "1";
        }
    };

    return async (rootElement) => {
        textNodeMap.clear();
        translationBuffers.clear();

        const textNodes = collectTextNodes(rootElement, textNodeMap);

        if (textNodes.length === 0) {
            return;
        }

        const messageListener = (message) => {
            if (message.type === "dom:update") {
                const node = textNodeMap.get(message.id);
                if (node && node.parentElement) {
                    addTranslatingEffect(node);

                    const currentText = translationBuffers.get(message.id) ?? "";
                    const newText = currentText + message.text;
                    translationBuffers.set(message.id, newText);

                    if (keepOriginal) {
                        const originalText = textNodes.find(n => n.id === message.id)?.text ?? "";
                        node.textContent = originalText + " / " + newText;
                    } else {
                        node.textContent = newText;
                    }
                }
            } else if (message.type === "dom:done") {
                const node = textNodeMap.get(message.id);
                if (node && node.parentElement) {
                    removeTranslatingEffect(node);

                    translationBuffers.delete(message.id);
                }
            } else if (message.type === "dom:complete") {
                textNodeMap.clear();
                translationBuffers.clear();
                chrome.runtime.onMessage.removeListener(messageListener);
            } else if (message.type === "dom:error") {
                console.error(message.error);
                textNodeMap.clear();
                translationBuffers.clear();
                chrome.runtime.onMessage.removeListener(messageListener);
            }
        };

        chrome.runtime.onMessage.addListener(messageListener);

        callback(textNodes);
    };
};
