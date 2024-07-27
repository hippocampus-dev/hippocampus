chrome.storage.sync.get(['enableStyler', 'apiKey'], function (items) {
    document.getElementById('enable-styler').checked = items.enableStyler;
    document.getElementById('api-key').value = items.apiKey;
});

document.getElementById('options-form').addEventListener('submit', (event) => {
    event.preventDefault();

    const enableStyler = document.getElementById('enable-styler').checked;
    const apiKey = document.getElementById('api-key').value;

    const items = {enableStyler};
    if (apiKey) {
        items.apiKey = apiKey;
    }

    chrome.storage.sync.set(items, () => {
        alert('Options saved');
    });
});
