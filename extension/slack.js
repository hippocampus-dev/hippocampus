(() => {
    let emojiMap = {};
    let prefixKey = 'ctrl';

    const loadSettings = () => {
        chrome.storage.sync.get(['slackEmojiMap', 'slackPrefixKey'], function (items) {
            emojiMap = items.slackEmojiMap || {};
            prefixKey = items.slackPrefixKey || 'ctrl';
        });
    };

    loadSettings();

    chrome.storage.onChanged.addListener((changes, namespace) => {
        if (namespace === 'sync' && (changes.slackEmojiMap || changes.slackPrefixKey)) {
            loadSettings();
        }
    });

    const getLastMessage = () => {
        const allMessages = document.querySelectorAll('[data-qa="virtual-list-item"]>[data-qa="message_container"]');
        if (allMessages.length > 0) {
            return allMessages[allMessages.length - 1];
        }
        return null;
    };

    const addReaction = async (messageElement, emoji) => {
        const rightClickEvent = new MouseEvent('contextmenu', {
            bubbles: true,
            cancelable: true,
            view: window,
            button: 2,
            buttons: 2
        });

        messageElement.dispatchEvent(rightClickEvent);

        await new Promise(resolve => setTimeout(resolve, 200));

        const contextMenuItems = document.querySelectorAll('[data-qa="menu_item_button"], [role="menuitem"]');

        const reactionMenuItem = Array.from(contextMenuItems).find((item) => {
            return item.getAttribute('data-qa') === 'add_reaction'
        });

        if (!reactionMenuItem) {
            return;
        }

        reactionMenuItem.click();

        await new Promise(resolve => setTimeout(resolve, 200));

        setTimeout(() => {
            const emojiPicker = document.querySelector('[data-qa="emoji_picker_input"]');
            if (!emojiPicker) return;

            emojiPicker.value = `:${emoji}:`;
            emojiPicker.dispatchEvent(new Event('input', {bubbles: true}));

            setTimeout(() => {
                const firstEmoji = document.querySelector('[data-qa="emoji_list_item"]:first-child');
                if (firstEmoji) {
                    firstEmoji.click();
                }
            }, 100);
        }, 100);

        return true;
    };

    document.addEventListener('keydown', async (event) => {
        const prefixSatisfied =
            (prefixKey === 'ctrl' && event.ctrlKey) ||
            (prefixKey === 'alt' && event.altKey) ||
            (prefixKey === 'ctrl+shift' && event.ctrlKey && event.shiftKey) ||
            (prefixKey === 'ctrl+alt' && event.ctrlKey && event.altKey) ||
            (prefixKey === 'alt+shift' && event.altKey && event.shiftKey) ||
            (prefixKey === 'ctrl+shift+alt' && event.ctrlKey && event.shiftKey && event.altKey);

        if (prefixSatisfied) {
            const emoji = emojiMap[event.key.toLowerCase()];
            if (emoji) {
                event.preventDefault();
                const messageElement = getLastMessage();
                if (messageElement) {
                    await addReaction(messageElement, emoji);
                }
            }
        }
    });
})();
