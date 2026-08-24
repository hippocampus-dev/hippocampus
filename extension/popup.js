document.addEventListener("DOMContentLoaded", () => {
  const reloadButton = document.getElementById("reload-extension");

  reloadButton.addEventListener("click", () => {
    chrome.runtime.reload();
  });
});
