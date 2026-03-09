import { useState, useEffect, useCallback, useRef } from "react";
import type { RunState, ServerMessage } from "@/lib/types";

const initialState: RunState = {
  status: "idle",
  mode: "infinite",
  targetCount: 1,
  completedCount: 0,
  currentPromptId: null,
  workflowName: "",
  progress: null,
  errors: [],
};

export function useAutoRun() {
  const [runState, setRunState] = useState<RunState>(initialState);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    function connect() {
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const ws = new WebSocket(
        `${protocol}//${window.location.host}/api/ws`,
      );

      ws.onopen = () => {
        setConnected(true);
      };

      ws.onmessage = (event) => {
        try {
          const message: ServerMessage = JSON.parse(event.data);
          if (message.type === "run_state") {
            setRunState(message.data);
          } else if (message.type === "progress") {
            setRunState((prev) => ({
              ...prev,
              progress: {
                current: message.data.current,
                total: message.data.total,
              },
            }));
          } else if (message.type === "completed") {
            setRunState((prev) => ({
              ...prev,
              completedCount: message.data.completedCount,
            }));
          }
        } catch {}
      };

      ws.onclose = () => {
        setConnected(false);
        wsRef.current = null;
        reconnectTimeoutRef.current = setTimeout(connect, 2000);
      };

      ws.onerror = () => {
        ws.close();
      };

      wsRef.current = ws;
    }

    connect();

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  const start = useCallback(
    async (options: {
      workflow: unknown;
      workflowName: string;
      mode: "infinite" | "count";
      count?: number;
      concept?: string;
      topologyHash?: string;
    }) => {
      const response = await fetch("/api/run/start", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          workflow: options.workflow,
          workflowName: options.workflowName,
          mode: options.mode,
          count: options.count,
          concept: options.concept,
          topologyHash: options.topologyHash,
        }),
      });
      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.message || "Failed to start");
      }
    },
    [],
  );

  const stop = useCallback(async () => {
    const response = await fetch("/api/run/stop", { method: "POST" });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.message || "Failed to stop");
    }
  }, []);

  return { runState, connected, start, stop };
}
