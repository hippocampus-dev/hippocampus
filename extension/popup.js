document.addEventListener('DOMContentLoaded', function() {
    const reloadButton = document.getElementById('reload-extension');

    reloadButton.addEventListener('click', function() {
        chrome.runtime.reload();
    });
});
