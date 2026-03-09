(() => {
    class EntryType {
        static #shortAnswer = 0;
        static #paragraph = 1;
        static #multipleChoice = 2;
        static #checkboxes = 3;
        static #dropdown = 4;
        static #fileUpload = 5;
        static #linearScale = 6;
        static #multipleChoiceGrid = 7;
        static #checkboxGrid = 8;
        static #date = 9;
        static #time = 10;

        static toString(value) {
            switch (value) {
                case this.#shortAnswer:
                    return "Short answer";
                case this.#paragraph:
                    return "Paragraph";
                case this.#multipleChoice:
                    return "Multiple choice";
                case this.#checkboxes:
                    return "Checkboxes";
                case this.#dropdown:
                    return "Dropdown";
                case this.#fileUpload:
                    return "File upload";
                case this.#linearScale:
                    return "Linear scale";
                case this.#multipleChoiceGrid:
                    return "Multiple choice grid";
                case this.#checkboxGrid:
                    return "Checkbox grid";
                case this.#date:
                    return "Date";
                case this.#time:
                    return "Time";
            }
        }

        static fillable(value) {
            switch (value) {
                case this.#shortAnswer:
                case this.#paragraph:
                case this.#multipleChoice:
                case this.#checkboxes:
                case this.#dropdown:
                    return true;
                default:
                    return false;
            }
        }

        static get SHORT_ANSWER() {
            return this.#shortAnswer;
        }

        static get PARAGRAPH() {
            return this.#paragraph;
        }

        static get MULTIPLE_CHOICE() {
            return this.#multipleChoice;
        }

        static get CHECKBOXES() {
            return this.#checkboxes;
        }

        static get DROPDOWN() {
            return this.#dropdown;
        }

        static get FILE_UPLOAD() {
            return this.#fileUpload;
        }

        static get LINEAR_SCALE() {
            return this.#linearScale;
        }

        static get MULTIPLE_CHOICE_GRID() {
            return this.#multipleChoiceGrid;
        }

        static get CHECKBOX_GRID() {
            return this.#checkboxGrid;
        }

        static get DATE() {
            return this.#date;
        }

        static get TIME() {
            return this.#time;
        }
    }

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

    monitor(`form`, (nodes) => {
        [...nodes]
            .forEach((node) => {
                for (const element of document.getElementsByTagName("extension-tooltip-root")) {
                    element.remove();
                }

                const range = new Range();
                range.selectNode(node);
                const rect = {
                    left: `${range.getBoundingClientRect().left}px`,
                    bottom: "calc(var(--tooltip-size))",
                };

                const optsBuilder = new TooltipOptsBuilder().withEphemeral(false);
                const tooltip = createTooltip(rect, (_self) => {
                    const scripts = document.querySelectorAll('script');
                    for (const script of scripts) {
                        const scriptText = script.innerHTML;
                        const data = scriptText.match(/FB_PUBLIC_LOAD_DATA_ = (.+);/);
                        if (data === null) {
                            continue;
                        }
                        const formData = JSON.parse(data[1]);

                        const formEntries = formData[1][1];

                        const content = {
                            url: window.location.href,
                            entries: [],
                        };
                        for (const entry of formEntries) {
                            const entryName = entry[1];
                            const entryType = entry[3];
                            if (!EntryType.fillable(entryType)) {
                                continue;
                            }
                            for (const entryValue of entry[4]) {
                                const entryId = entryValue[0];
                                const entryValueOptions = entryValue[1];

                                if (entryValueOptions === null) {
                                    content.entries.push({
                                        id: entryId,
                                        name: entryName,
                                        type: EntryType.toString(entryType),
                                    });
                                    continue;
                                }

                                const options = [];
                                for (const entryValueOption of entryValueOptions) {
                                    options.push(entryValueOption[0]);
                                }
                                content.entries.push({
                                    id: entryId,
                                    name: entryName,
                                    type: EntryType.toString(entryType),
                                    options,
                                });
                            }
                        }
                        chrome.runtime.sendMessage({
                            type: "autofill",
                            content: content,
                        });
                    }
                }, optsBuilder.build());

                document.querySelectorAll(".tooltip-anchor").forEach((element) => element.classList.remove("tooltip-anchor"));
                node.classList.add("tooltip-anchor");

                tooltip.style.positionAnchor = "--tooltip";
                tooltip.style.positionArea = "top left";

                document.body.after(tooltip);
            });
    });
})();
