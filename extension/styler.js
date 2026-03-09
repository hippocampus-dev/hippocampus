(() => {
    const config = {
        "all": {
            register: (callback) => {
                callback();
            },
            css: () => {
                const anchors = [
                    ".dialog-anchor { anchor-name: --dialog; }",
                    ".tooltip-anchor { anchor-name: --tooltip; }",
                    ".menu-anchor { anchor-name: --menu; }",
                ];
                return anchors.join("\n");
            },
        },
        "my-github": {
            register: (callback) => {
                if (location.href.startsWith("https://github.com/kaidotio")) {
                    callback();
                }
            },
            files: ["styler/github.css"],
        },
        "local-grafana": {
            register: (callback) => {
                if (location.href.startsWith("https://grafana.127.0.0.1.nip.io")) {
                    callback();
                }
            },
            files: ["styler/local-grafana.css"],
        },
        "aws": {
            register: (callback) => {
                if (!location.hostname.endsWith(".console.aws.amazon.com")) {
                    return;
                }
                new MutationObserver((_records, observer) => {
                    const element = document.querySelector(
                        "#menu--account > div > div > div:nth-child(1) > span:nth-child(2)"
                    );
                    if (element !== null) {
                        const accountId = element.textContent.replaceAll("-", "");
                        if (accountId.startsWith("123456789012")) {
                            callback();
                        }
                        observer.disconnect();
                    }
                }).observe(document.documentElement, {
                    childList: true,
                    subtree: true,
                });
            },
            css: () => {
                return "styler/aws.css";
            }
        },
    };

    for (const c in config) {
        config[c].register(() => {
            chrome.runtime.sendMessage({
                type: "styler",
                css: config[c].css ? config[c].css() : undefined,
                files: config[c].files,
            });
        });
    }
})();
