import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  connectNativeHost,
  HOST_NAME,
  NativeMessagingRouter,
} from "../../lib/native_messaging.js";

type MessageListener = (message: unknown) => void;
type DisconnectListener = () => void;

class MockPort {
  hostName: string;
  posted: unknown[] = [];
  disconnected = false;
  #messageListeners: MessageListener[] = [];
  #disconnectListeners: DisconnectListener[] = [];

  constructor(hostName: string) {
    this.hostName = hostName;
  }

  onMessage = {
    addListener: (listener: MessageListener) => {
      this.#messageListeners.push(listener);
    },
  };

  onDisconnect = {
    addListener: (listener: DisconnectListener) => {
      this.#disconnectListeners.push(listener);
    },
  };

  postMessage(message: unknown) {
    if (this.disconnected) {
      throw new Error("Attempting to use a disconnected port object");
    }
    this.posted.push(message);
  }

  disconnect() {
    this.disconnected = true;
  }

  emitMessage(message: unknown) {
    for (const listener of this.#messageListeners) {
      listener(message);
    }
  }

  emitDisconnect() {
    this.disconnected = true;
    for (const listener of this.#disconnectListeners) {
      listener();
    }
  }
}

let ports: MockPort[] = [];

beforeEach(() => {
  vi.useFakeTimers();
  ports = [];
  Object.assign(globalThis, {
    chrome: {
      runtime: {
        lastError: undefined,
        connectNative: (hostName: string) => {
          const port = new MockPort(hostName);
          ports.push(port);
          return port;
        },
      },
    },
  });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("NativeMessagingRouter", () => {
  test("dispatches registered sync and async handlers", async () => {
    const router = new NativeMessagingRouter()
      .register("echo", (message: { value: number }) => ({
        echoed: message.value,
      }))
      .register("async", async () => ({ ok: true }));

    await expect(
      router.dispatch({ type: "echo", value: 42 }),
    ).resolves.toStrictEqual({ echoed: 42 });
    await expect(router.dispatch({ type: "async" })).resolves.toStrictEqual({
      ok: true,
    });
  });

  test("rejects missing and unknown message types", async () => {
    const router = new NativeMessagingRouter();

    await expect(router.dispatch({})).rejects.toThrow("missing message type");
    await expect(router.dispatch(null)).rejects.toThrow("missing message type");
    await expect(router.dispatch({ type: "unknown" })).rejects.toThrow(
      "unknown message type: unknown",
    );
  });
});

describe("connectNativeHost", () => {
  test("sends connect handshake and replies to requests", async () => {
    const pushReceived: unknown[] = [];
    const router = new NativeMessagingRouter()
      .register("echo", (message: { value: number }) => ({
        echoed: message.value,
      }))
      .register("void", () => undefined)
      .register("fail", () => {
        throw new Error("boom");
      })
      .register("notify", (message: unknown) => {
        pushReceived.push(message);
      });

    const client = connectNativeHost(router);
    expect(ports).toHaveLength(1);
    const port = ports[0];
    expect(port.hostName).toBe(HOST_NAME);
    expect(port.posted).toStrictEqual([{ type: "connect" }]);

    port.emitMessage({});
    await vi.runAllTicks();
    expect(port.posted).toHaveLength(1);

    port.emitMessage({ id: 1, type: "echo", value: 42 });
    port.emitMessage({ id: 2, type: "void" });
    port.emitMessage({ id: 3, type: "fail" });
    port.emitMessage({ id: 4, type: "missing" });
    port.emitMessage({ type: "notify", text: "hello" });
    await vi.waitFor(() => {
      expect(port.posted).toHaveLength(5);
    });

    const replies = (port.posted.slice(1) as Array<{ id: number }>).sort(
      (left, right) => left.id - right.id,
    );
    expect(replies).toStrictEqual([
      { id: 1, result: { echoed: 42 } },
      { id: 2, result: null },
      { id: 3, error: "boom" },
      { id: 4, error: "unknown message type: missing" },
    ]);
    expect(pushReceived).toStrictEqual([{ type: "notify", text: "hello" }]);
    client.disconnect();
  });

  test("backs off and stops retrying when host never responds", () => {
    const router = new NativeMessagingRouter();
    const client = connectNativeHost(router);

    const delays = [1000, 2000, 4000, 8000, 16000];
    for (let index = 0; index < delays.length; index++) {
      ports[index].emitDisconnect();
      vi.advanceTimersByTime(delays[index] - 1);
      expect(ports).toHaveLength(index + 1);
      vi.advanceTimersByTime(1);
      expect(ports).toHaveLength(index + 2);
    }

    ports[delays.length].emitDisconnect();
    vi.advanceTimersByTime(60000);
    expect(ports).toHaveLength(delays.length + 1);

    client.reconnect();
    expect(ports).toHaveLength(delays.length + 2);
    client.disconnect();
  });

  test("resets reconnect backoff after receiving a message", () => {
    const router = new NativeMessagingRouter();
    const client = connectNativeHost(router);

    ports[0].emitDisconnect();
    vi.advanceTimersByTime(1000);
    expect(ports).toHaveLength(2);

    ports[1].emitMessage({});
    ports[1].emitDisconnect();
    vi.advanceTimersByTime(1000);
    expect(ports).toHaveLength(3);
    client.disconnect();
  });

  test("skips posting reply if port disconnects before handler resolves", async () => {
    let resolveHandler: ((value: unknown) => void) | null = null;
    const router = new NativeMessagingRouter().register(
      "slow",
      () =>
        new Promise((resolve) => {
          resolveHandler = resolve;
        }),
    );

    const client = connectNativeHost(router);
    const port = ports[0];
    port.emitMessage({ id: 1, type: "slow" });
    port.emitDisconnect();
    resolveHandler?.({ ok: true });
    await vi.runAllTicks();

    expect(port.posted).toStrictEqual([{ type: "connect" }]);
    client.disconnect();
  });

  test("reconnects after disconnect and catches postMessage errors on disconnected port", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const router = new NativeMessagingRouter().register("echo", () => ({
      ok: true,
    }));

    const client = connectNativeHost(router);
    ports[0].disconnected = true;
    ports[0].emitMessage({ id: 1, type: "echo" });
    await vi.runAllTicks();
    expect(errorSpy).toHaveBeenCalledTimes(1);

    client.disconnect();
    client.reconnect();
    expect(ports).toHaveLength(2);
    expect(ports[1].posted).toStrictEqual([{ type: "connect" }]);

    client.disconnect();
    errorSpy.mockRestore();
  });
});
