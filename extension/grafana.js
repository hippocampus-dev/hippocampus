(() => {
    const filterTextNodes = (node, cb) => {
        if (node.nodeType === Node.TEXT_NODE) {
            return [node];
        }

        let textNodes = [];
        if (cb(node)) {
            node.childNodes?.forEach((childNode) => {
                textNodes = textNodes.concat(filterTextNodes(childNode, cb));
            });
        }
        return textNodes;
    }

    const findParentElement = (element, cb) => {
        const parent = element.parentElement;
        if (parent === null || cb(parent)) {
            return parent;
        }
        return findParentElement(parent, cb);
    }

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

    const RFC3339_REGEXP = /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:.\d+)?(?:Z|[+-]\d{2}:\d{2})/g;
    const PYTHON_STACKTRACE_REGEXP = /File "([a-zA-Z0-9/_-]+\.[a-zA-Z]+)", line (\d+)/g;
    const STACKTRACE_REGEXP = /([@.a-zA-Z0-9/_-]+\.[a-zA-Z]+):(\d+)/g;
    const ERROR_REGEXP = /warning|error|critical|alert|emergency/gi;

    class Left {
        constructor(url) {
            const param = url.searchParams.get("left");
            if (param === null) {
                return;
            }
            const json = JSON.parse(param);
            this.datasource = json.datasource;
            this.queries = json.queries;
            this.range = json.range;
        }
    }

    const GRAFANA_LOGS_DETAILS_TABLE_CLASS_NAME = "css-1s7novq-logs-row-details-table";
    const GRAFANA_LOGS_DETAIL_LABEL_SELECTOR = ".css-1d24gbv-logs-row-details__label";

    const fetchLogLabelsAndDetectedFields = (node) => {
        const m = {};
        const buttons = {};

        const parent = findParentElement(node, (element) => {
            return element.tagName === "DIV" && element.className === GRAFANA_LOGS_DETAILS_TABLE_CLASS_NAME;
        });
        if (parent === null) {
            return m;
        }

        parent.querySelectorAll(GRAFANA_LOGS_DETAIL_LABEL_SELECTOR).forEach((element) => {
            const childNodes = element.nextElementSibling.childNodes[0].childNodes;
            m[element.textContent] = childNodes[0];
            // This is a button
            if (childNodes[2] !== undefined) {
                buttons[element.textContent] = childNodes[2];
            }
        });

        return m;
    }

    const showLinkify = (left, node, match) => {
        const logLabelsAndDetectedFields = fetchLogLabelsAndDetectedFields(node);
        if (Object.keys(logLabelsAndDetectedFields).length === 0) {
            return match;
        }

        const timestamp = parseInt(Date.parse(logLabelsAndDetectedFields["time"].textContent));
        const query = encodeURIComponent(`{"datasource":"Loki","queries":[{"refId":"A","expr":"{grouping=\\"${logLabelsAndDetectedFields["grouping"].textContent}\\"} | pattern \\"<raw>\\" | json | kubernetes_pod_name = \\"${logLabelsAndDetectedFields["kubernetes_pod_name"].textContent}\\" | line_format \\"{{ $structural_message := index (fromJson $.raw) \\\\\\"structural_message\\\\\\" }}{{ if $structural_message }}{{ range $k, $v := $structural_message }}\\\\033[1;33m{{ $k }}\\\\033[0m={{ $v }}\\\\n{{ end }}{{ else }}{{ $.message }}{{ end }}\\"","queryTime":"range","maxLines":1000}],"range":{"from":"${timestamp - 1000}","to":"${timestamp - 1000}"}}`)
        const hl = encodeURIComponent(match);
        return `<a target="_blank" style="text-decoration: underline;" href="/explore?orgId=1&left=${query}&hl=${hl}">${match}</a>`;
    }

    const gitHubLinkify = (left, node, match, filepath, lineno) => {
        const logLabelsAndDetectedFields = fetchLogLabelsAndDetectedFields(node);
        if (Object.keys(logLabelsAndDetectedFields).length === 0) {
            return match;
        }

        if (filepath.startsWith("/")) { // Absolute Path
            if (filepath.startsWith("/opt")) { // see: https://refspecs.linuxfoundation.org/FHS_3.0/fhs/index.html
                const shards = filepath.split("/");
                return `<a target="_blank" style="text-decoration: underline;" href="https://github.com/hippocampus-dev/hippocampus/blob/HEAD/cluster/applications/${shards.slice(3, shards.length).join("/")}#L${lineno}">${match}</a>`;
            }
        } else { // Relative Path
            if (filepath.includes("@")) { // Remote Repository
                const shards = filepath.split("/");
                const reporevision = [shards.shift(), shards.shift()].join("/").split("@");
                return `<a target="_blank" style="text-decoration: underline;" href="https://github.com/${reporevision[0]}/blob/${reporevision[1]}/${shards.join("/")}#L${lineno}">${match}</a>`;
            }

            const repositoryNode = logLabelsAndDetectedFields["repository"]
            if (repositoryNode === undefined) {
                return match;
            }
            const repository = repositoryNode.textContent;
            const ownername = repository.split("/")[0];

            if (filepath.startsWith(ownername[1])) {
                return `<a target="_blank" style="text-decoration: underline;" href="https://github.com/${repository}/blob/HEAD/${filepath.slice(ownername[1].length + 1)}#L${lineno}">${match}</a>`;
            } else {
                return `<a target="_blank" style="text-decoration: underline;" href="https://github.com/${repository}/blob/HEAD/${filepath}#L${lineno}">${match}</a>`;
            }
        }
    }

    const explainMessageTooltip = (left, node, match) => {
        const logLabelsAndDetectedFields = fetchLogLabelsAndDetectedFields(node);
        if (Object.keys(logLabelsAndDetectedFields).length === 0) {
            return;
        }

        for (const element of document.getElementsByTagName("extension-tooltip-root")) {
            element.remove();
        }

        const messageNode = logLabelsAndDetectedFields["message"];
        if (messageNode === undefined) {
            return;
        }

        const rect = {
            left: "calc(var(--tooltip-size) * -1)",
            bottom: "0px",
        };

        const optsBuilder = new TooltipOptsBuilder().withEphemeral(false);
        const tooltip = createTooltip(rect, () => {
            chrome.runtime.sendMessage({
                type: "suggest",
                content: messageNode.textContent,
            });
        }, optsBuilder.build());

        document.querySelectorAll(".tooltip-anchor").forEach((element) => element.classList.remove("tooltip-anchor"));
        messageNode.parentElement.classList.add("tooltip-anchor");

        tooltip.style.positionAnchor = "--tooltip";
        tooltip.style.insetArea = "bottom right";

        document.body.after(tooltip);
    }

    class Ticker {
        constructor(handler, max) {
            this.handler = handler;
            this.max = max;
            this.timer = undefined;
            this.tick = 0;
        }

        setTimer(timer) {
            this.stopTimer();
            this.timer = setTimeout(() => {
                this.tick++;
                if (this.max >= this.tick) {
                    this.handler();
                }
            }, timer);
        }

        stopTimer() {
            if (this.timer !== undefined) {
                clearTimeout(this.timer);
            }
        }
    }

    const autoScroll = new Ticker(() => {
        const button = document.querySelector(`[data-testid="olderLogsButton"]`)[0];
        if (button !== undefined) {
            button.click();
        }
    }, 10);

    let scrolled = false;

    const autoScrollEnabled = () => {
        return new URL(location.href).searchParams.get("hl") !== null && !scrolled;
    }

    const disableAutoScroll = () => {
        scrolled = true;
    }

    monitor("div, tr", (nodes) => {
        const url = new URL(location.href);
        const left = new Left(url);
        const hl = url.searchParams.get("hl");

        [...nodes]
            .forEach((node) => {
                const textNodes = filterTextNodes(node, (node) => {
                    return node.tagName !== "A" && node.className !== "slate-query-field";
                });
                textNodes.forEach((textNode) => {
                    if (autoScrollEnabled() && textNode.textContent.match(new RegExp(hl))) {
                        const parent = findParentElement(textNode, (element) => {
                            return element.tagName === "DIV";
                        });
                        if (parent !== null) {
                            parent.scrollIntoView();
                            parent.style.border = "solid";
                        }

                        disableAutoScroll();
                        autoScroll.stopTimer();
                    }

                    const boundShowLinkify = showLinkify.bind(null, left, textNode);
                    const boundGitHubLinkify = gitHubLinkify.bind(null, left, textNode);
                    const boundExplainMessageTooltip = explainMessageTooltip.bind(null, left, textNode);

                    const result = textNode.textContent
                        .replace(RFC3339_REGEXP, boundShowLinkify)
                        .replace(PYTHON_STACKTRACE_REGEXP, boundGitHubLinkify)
                        .replace(STACKTRACE_REGEXP, boundGitHubLinkify);

                    if (ERROR_REGEXP.test(textNode.textContent)) {
                        boundExplainMessageTooltip();
                    }

                    if (textNode.textContent !== result) {
                        const element = document.createElement("span");
                        element.innerHTML = result;
                        textNode.parentNode.insertBefore(element, textNode);
                        textNode.parentNode.removeChild(textNode);
                    }
                });
            });

        if (autoScrollEnabled()) {
            // MutationObserver executes callback concurrently.
            // Delayed auto scrolling so that it can be canceled from the outside.
            autoScroll.setTimer(100);
        }
    });
})();
