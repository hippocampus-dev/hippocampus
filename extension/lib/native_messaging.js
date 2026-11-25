export const HOST_NAME = "dev.kaidotio.taurin";
const INITIAL_RECONNECT_DELAY_MILLISECONDS = 1000;
const MAX_RECONNECT_DELAY_MILLISECONDS = 16000;
const MAX_RECONNECT_ATTEMPTS = 5;

export class NativeMessagingRouter {
  #handlers = new Map();

  register(messageType, handler) {
    this.#handlers.set(messageType, handler);
    return this;
  }

  async dispatch(message) {
    if (
      message === null ||
      typeof message !== "object" ||
      typeof message.type !== "string"
    ) {
      throw new Error("missing message type");
    }
    const handler = this.#handlers.get(message.type);
    if (handler === undefined) {
      throw new Error(`unknown message type: ${message.type}`);
    }
    return handler(message);
  }
}

export function connectNativeHost(router, hostName = HOST_NAME) {
  let port = null;
  let reconnectTimer = null;
  let reconnectAttempts = 0;
  let stopped = false;

  const connect = () => {
    if (stopped || port !== null) {
      return;
    }

    const currentPort = chrome.runtime.connectNative(hostName);
    port = currentPort;

    currentPort.onMessage.addListener(async (message) => {
      reconnectAttempts = 0;
      if (
        message === null ||
        typeof message !== "object" ||
        typeof message.type !== "string"
      ) {
        return;
      }

      const requestId = message.id;
      const sendReply = (reply) => {
        if (requestId === undefined || port !== currentPort) {
          return;
        }
        try {
          currentPort.postMessage(reply);
        } catch (postError) {
          console.error(postError);
        }
      };
      try {
        const result = await router.dispatch(message);
        sendReply({
          id: requestId,
          result: result === undefined ? null : result,
        });
      } catch (error) {
        if (requestId !== undefined) {
          sendReply({
            id: requestId,
            error: error instanceof Error ? error.message : String(error),
          });
        } else {
          console.error(error);
        }
      }
    });

    currentPort.onDisconnect.addListener(() => {
      // https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging#native-messaging-client
      void chrome.runtime.lastError;
      if (port !== currentPort) {
        return;
      }
      port = null;
      if (stopped || reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
        return;
      }
      const delay = Math.min(
        INITIAL_RECONNECT_DELAY_MILLISECONDS * 2 ** reconnectAttempts,
        MAX_RECONNECT_DELAY_MILLISECONDS,
      );
      reconnectAttempts++;
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connect();
      }, delay);
    });

    currentPort.postMessage({ type: "connect" });
  };

  connect();

  return {
    reconnect() {
      stopped = false;
      reconnectAttempts = 0;
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      connect();
    },
    disconnect() {
      stopped = true;
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      if (port !== null) {
        const currentPort = port;
        port = null;
        currentPort.disconnect();
      }
    },
  };
}
