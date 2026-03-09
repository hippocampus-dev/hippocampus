chrome.storage.sync.get(['enableStyler', 'apiKey', 'slackPrefixKey', 'slackEmojiMap', 'blockedChannels', 'autoCloseUrlPatterns'], function (items) {
    document.getElementById('enable-styler').checked = items.enableStyler;
    document.getElementById('api-key').value = items.apiKey || '';
    document.getElementById('slack-prefix-key').value = items.slackPrefixKey || 'ctrl';

    const emojiMap = items.slackEmojiMap || {};
    const bindingsContainer = document.getElementById('slack-emoji-bindings');

    Object.entries(emojiMap).forEach(([key, emoji]) => {
        const bindingDiv = document.createElement('div');
        bindingDiv.style.cssText = 'display: grid; grid-template-columns: 50px 200px 80px; gap: 10px; margin-bottom: 5px;';

        const keyInput = document.createElement('input');
        keyInput.type = 'text';
        keyInput.value = key;
        keyInput.maxLength = 1;
        keyInput.style.width = '40px';
        keyInput.className = 'slack-key-input';

        const emojiInput = document.createElement('input');
        emojiInput.type = 'text';
        emojiInput.value = emoji;
        emojiInput.placeholder = 'emoji name (e.g., thumbsup)';
        emojiInput.className = 'slack-emoji-input';

        const removeButton = document.createElement('button');
        removeButton.type = 'button';
        removeButton.textContent = 'Remove';
        removeButton.onclick = () => bindingDiv.remove();

        bindingDiv.appendChild(keyInput);
        bindingDiv.appendChild(emojiInput);
        bindingDiv.appendChild(removeButton);

        bindingsContainer.appendChild(bindingDiv);
    });

    const blockedChannels = items.blockedChannels || [];
    const youtubeContainer = document.getElementById('youtube-blocked-channels');

    blockedChannels.forEach((channel) => {
        const channelDiv = document.createElement('div');
        channelDiv.style.cssText = 'display: flex; gap: 10px; margin-bottom: 5px; align-items: center; max-width: 400px;';

        const channelSpan = document.createElement('span');
        channelSpan.textContent = channel;
        channelSpan.style.cssText = 'padding: 5px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0;';

        const unblockButton = document.createElement('button');
        unblockButton.type = 'button';
        unblockButton.textContent = 'Unblock';
        unblockButton.setAttribute('data-channel', channel);
        unblockButton.setAttribute('title', channel);
        unblockButton.style.flexShrink = '0';
        unblockButton.onclick = () => {
            chrome.storage.sync.get(['blockedChannels'], function (items) {
                const channels = items.blockedChannels || [];
                const updatedChannels = channels.filter(c => c !== channel);
                chrome.storage.sync.set({blockedChannels: updatedChannels}, () => {
                    channelDiv.remove();
                });
            });
        };

        channelDiv.appendChild(channelSpan);
        channelDiv.appendChild(unblockButton);
        youtubeContainer.appendChild(channelDiv);
    });

    if (blockedChannels.length === 0) {
        const emptyMessage = document.createElement('p');
        emptyMessage.textContent = 'No blocked channels';
        emptyMessage.style.color = '#666';
        youtubeContainer.appendChild(emptyMessage);
    }

    const urlPatterns = items.autoCloseUrlPatterns || [];
    const urlPatternsContainer = document.getElementById('url-patterns-list');

    const renderUrlPatterns = () => {
        urlPatternsContainer.innerHTML = '';

        if (urlPatterns.length === 0) {
            const emptyMessage = document.createElement('p');
            emptyMessage.textContent = 'No URL patterns configured';
            emptyMessage.style.cssText = 'color: #666; font-size: 14px;';
            urlPatternsContainer.appendChild(emptyMessage);
        } else {
            urlPatterns.forEach((pattern) => {
                const patternDiv = document.createElement('div');
                patternDiv.style.cssText = 'display: flex; gap: 10px; margin-bottom: 5px; align-items: center; max-width: 400px;';

                const patternSpan = document.createElement('span');
                patternSpan.textContent = pattern;
                patternSpan.style.cssText = 'padding: 5px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; font-family: monospace; background: #f0f0f0; border-radius: 3px;';

                const removeButton = document.createElement('button');
                removeButton.type = 'button';
                removeButton.textContent = 'Remove';
                removeButton.style.flexShrink = '0';
                removeButton.onclick = () => {
                    const index = urlPatterns.indexOf(pattern);
                    if (index > -1) {
                        urlPatterns.splice(index, 1);
                        renderUrlPatterns();
                        // Save immediately after removing
                        chrome.storage.sync.set({autoCloseUrlPatterns: urlPatterns});
                    }
                };

                patternDiv.appendChild(patternSpan);
                patternDiv.appendChild(removeButton);
                urlPatternsContainer.appendChild(patternDiv);
            });
        }
    };

    renderUrlPatterns();

    document.getElementById('add-url-pattern').addEventListener('click', () => {
        const input = document.getElementById('new-url-pattern');
        const pattern = input.value.trim();

        if (pattern && !urlPatterns.includes(pattern)) {
            urlPatterns.push(pattern);
            input.value = '';
            renderUrlPatterns();
            // Save immediately after adding
            chrome.storage.sync.set({autoCloseUrlPatterns: urlPatterns});
        }
    });

    document.getElementById('new-url-pattern').addEventListener('keypress', (event) => {
        if (event.key === 'Enter') {
            event.preventDefault();
            document.getElementById('add-url-pattern').click();
        }
    });
});

document.getElementById('slack-add-binding').addEventListener('click', () => {
    const bindingsContainer = document.getElementById('slack-emoji-bindings');
    const bindingDiv = document.createElement('div');
    bindingDiv.style.cssText = 'display: grid; grid-template-columns: 50px 200px 80px; gap: 10px; margin-bottom: 5px;';

    const keyInput = document.createElement('input');
    keyInput.type = 'text';
    keyInput.value = '';
    keyInput.maxLength = 1;
    keyInput.style.width = '40px';
    keyInput.className = 'slack-key-input';

    const emojiInput = document.createElement('input');
    emojiInput.type = 'text';
    emojiInput.value = '';
    emojiInput.placeholder = 'emoji name (e.g., thumbsup)';
    emojiInput.className = 'slack-emoji-input';

    const removeButton = document.createElement('button');
    removeButton.type = 'button';
    removeButton.textContent = 'Remove';
    removeButton.onclick = () => bindingDiv.remove();

    bindingDiv.appendChild(keyInput);
    bindingDiv.appendChild(emojiInput);
    bindingDiv.appendChild(removeButton);

    bindingsContainer.appendChild(bindingDiv);
});

document.getElementById('options-form').addEventListener('submit', (event) => {
    event.preventDefault();

    const enableStyler = document.getElementById('enable-styler').checked;
    const apiKey = document.getElementById('api-key').value;
    const prefixKey = document.getElementById('slack-prefix-key').value;
    const emojiMap = {};

    document.querySelectorAll('.slack-key-input').forEach((keyInput, index) => {
        const emojiInput = document.querySelectorAll('.slack-emoji-input')[index];
        if (keyInput.value && emojiInput.value) {
            emojiMap[keyInput.value.toLowerCase()] = emojiInput.value;
        }
    });

    const urlPatterns = [];
    document.querySelectorAll('#url-patterns-list > div').forEach((patternDiv) => {
        const patternSpan = patternDiv.querySelector('span');
        if (patternSpan && patternSpan.textContent) {
            urlPatterns.push(patternSpan.textContent);
        }
    });

    const items = {enableStyler, slackPrefixKey: prefixKey, slackEmojiMap: emojiMap, autoCloseUrlPatterns: urlPatterns};
    if (apiKey) {
        items.apiKey = apiKey;
    }

    chrome.storage.sync.set(items, () => {
        alert('Options saved');
    });
});
