(() => {
    chrome.runtime.onMessage.addListener((message) => {
        switch (message.type) {
            case "dialog:create": {
                const dialogs = document.getElementsByTagName("extension-dialog-root");
                for (const element of dialogs) {
                    element.remove();
                }

                let rect = {
                    left: `${window.scrollX + 5}px`,
                    bottom: `${window.scrollY + 5}px`,
                };

                const selection = window.getSelection();
                if (!(selection === undefined || selection.toString().length === 0)) {
                    const range = selection.getRangeAt(0);

                    const end = range.endContainer.parentElement;
                    document.querySelectorAll(".dialog-anchor").forEach((element) => element.classList.remove("dialog-anchor"));
                    end.classList.add("dialog-anchor");

                    const baseRect = range.getBoundingClientRect();
                    const endRect = end.getBoundingClientRect();
                    rect = {
                        left: `${baseRect.left - endRect.left}px`,
                        bottom: `${baseRect.bottom - endRect.bottom + 5}px`,
                    };
                }

                const opts = new DialogOptsBuilder().withOriginal(message.original).build();
                const dialog = createDialog(rect, message.content, opts);

                dialog.style.positionAnchor = "--dialog";
                dialog.style.positionArea = "bottom span-right";

                document.body.after(dialog);
                break;
            }
            case "dialog:message": {
                const dialogs = document.getElementsByTagName("extension-dialog-root");
                if (dialogs.length === 0) {
                    chrome.runtime.sendMessage({
                        type: "abort",
                        content: "dialog:message",
                    });
                    return;
                }
                for (const element of dialogs) {
                    element.dispatchEvent(new CustomEvent("message", {
                        detail: message.content,
                    }));
                }
                break;
            }
        }
    });
})();
