const authURL = () => {
  return `https://bakery.kaidotio.dev/callback?cookie_name=_oauth2_proxy&redirect_url=${encodeURIComponent(chrome.identity.getRedirectURL("extension"))}`;
};

export const cookieFromWebAuthFlow = (tabId) => {
  return chrome.identity
    .launchWebAuthFlow({
      url: authURL(),
    })
    .then((responseURL) => {
      const url = new URL(responseURL);
      const value = "_oauth2_proxy=" + url.searchParams.get("value");
      const expires = url.searchParams.get("expires");
      return Promise.resolve({ value, expires });
    })
    .catch((error) => {
      if (error.toString().startsWith("Error: User interaction required.")) {
        return chrome.tabs
          .create({
            url: authURL(),
          })
          .then((tab) => {
            return new Promise((resolve) => {
              let i = 0;
              chrome.tabs.onUpdated.addListener(function listener(id, info) {
                if (info.status === "complete" && id === tab.id) {
                  i++;
                  if (i === 2) {
                    // HACK: Wait to redirect
                    chrome.tabs.onUpdated.removeListener(listener);
                    resolve(tab);
                  }
                }
              });
            });
          })
          .then((tab) => {
            return chrome.tabs.remove(tab.id);
          })
          .then(() => {
            return chrome.tabs.update(tabId, { highlighted: true });
          })
          .then(() => {
            return cookieFromWebAuthFlow(tabId);
          });
      } else {
        return chrome.tabs
          .reload(tabId)
          .then(() => {
            return new Promise((resolve) => {
              chrome.tabs.onUpdated.addListener(function listener(id, info) {
                if (info.status === "complete" && id === tabId) {
                  chrome.tabs.onUpdated.removeListener(listener);
                  resolve();
                }
              });
            });
          })
          .then(() => {
            return cookieFromWebAuthFlow(tabId);
          });
      }
    });
};
