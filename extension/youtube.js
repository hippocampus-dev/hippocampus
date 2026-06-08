(() => {
  const selector = `ytd-video-renderer`;

  let blockedChannels = [];

  const loadSettings = () => {
    chrome.storage.sync.get(["blockedChannels"], (items) => {
      blockedChannels = items.blockedChannels || [];
    });
  };

  loadSettings();

  chrome.storage.onChanged.addListener((changes, namespace) => {
    if (namespace === "sync" && changes.blockedChannels) {
      loadSettings();
    }
  });

  const block = (nodes) => {
    [...nodes].forEach((node) => {
      const channelElement = node.querySelector(
        `ytd-channel-name#channel-name > div.container > div.text-container > yt-formatted-string`,
      );
      if (!channelElement) {
        return;
      }

      const channelName = channelElement.textContent;

      if (blockedChannels.includes(channelName)) {
        node.style.display = "none";
        return;
      }

      if (node.querySelector(".channel-block-button")) {
        return;
      }

      const button = document.createElement("button");
      button.type = "button";
      button.className = "channel-block-button";
      button.setAttribute("data-channel-name", channelName);
      button.setAttribute("aria-label", "Block channel");
      button.textContent = "Block";

      button.style.marginLeft = "8px";
      button.style.padding = "2px 8px";
      button.style.fontSize = "12px";
      button.style.backgroundColor = "#cc0000";
      button.style.color = "white";
      button.style.border = "none";
      button.style.borderRadius = "4px";
      button.style.cursor = "pointer";
      button.style.fontFamily = "Roboto, Arial, sans-serif";
      button.style.fontWeight = "500";
      button.style.textTransform = "uppercase";
      button.style.display = "inline-block";
      button.style.verticalAlign = "middle";

      button.addEventListener("pointerover", () => {
        button.style.opacity = "0.8";
      });

      button.addEventListener("pointerout", () => {
        button.style.opacity = "1";
      });

      button.addEventListener("click", (event) => {
        event.preventDefault();
        event.stopPropagation();

        chrome.storage.sync.get(["blockedChannels"], (items) => {
          const channels = items.blockedChannels || [];
          channels.push(channelName);
          chrome.storage.sync.set({ blockedChannels: channels });
        });
      });

      const buttonsContainer = node.querySelector(
        "div#dismissible > div > div#buttons",
      );
      if (buttonsContainer) {
        buttonsContainer.appendChild(button);
      }
    });
  };

  block(document.querySelectorAll(selector));

  new MutationObserver((records) => {
    records.forEach((record) => {
      record.addedNodes.forEach((addedNode) => {
        if (addedNode.nodeType === Node.ELEMENT_NODE) {
          if (addedNode.matches && addedNode.matches(selector)) {
            block([addedNode]);
          } else {
            block(addedNode.querySelectorAll(selector));
          }
        }
      });
    });
  }).observe(document.body, {
    childList: true,
    subtree: true,
  });

  const pinStorageKey = (videoId) => `youtube-pin:${videoId}`;

  document.addEventListener(
    "keydown",
    (event) => {
      if (event.ctrlKey || event.metaKey || event.altKey) {
        return;
      }

      const target = event.target;
      if (target) {
        const tagName = target.tagName;
        if (
          tagName === "INPUT" ||
          tagName === "TEXTAREA" ||
          tagName === "SELECT"
        ) {
          return;
        }
        if (target.isContentEditable) {
          return;
        }
      }

      const videoId = new URLSearchParams(location.search).get("v");
      if (!videoId) {
        return;
      }

      const video =
        document.querySelector("video.html5-main-video") ||
        document.querySelector("video");
      if (!video) {
        return;
      }

      const key = pinStorageKey(videoId);

      if (event.key === "p") {
        chrome.storage.local.set({ [key]: video.currentTime });
        event.preventDefault();
      } else if (event.key === "o") {
        chrome.storage.local.get([key], (items) => {
          const time = items[key];
          if (typeof time === "number") {
            video.currentTime = time;
          }
        });
        event.preventDefault();
      }
    },
    true,
  );
})();
