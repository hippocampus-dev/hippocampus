import { h } from "https://cdn.skypack.dev/preact@10.22.1";
import {
  useEffect,
  useRef,
} from "https://cdn.skypack.dev/preact@10.22.1/hooks";
import { Terminal as XTerm } from "https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/+esm";
import { FitAddon } from "https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/+esm";

import { WS_HOST } from "../constants/host.js";

const Terminal = ({ namespace, pod, container }) => {
  const containerRef = useRef(null);

  useEffect(() => {
    if (containerRef.current === null) {
      return;
    }

    const term = new XTerm({ convertEol: true });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(containerRef.current);
    fitAddon.fit();

    const params = new URLSearchParams();
    if (container !== undefined && container !== "") {
      params.set("container", container);
    }
    const query = params.toString();
    const url = `${WS_HOST}/${namespace}/core/v1/pod/${pod}/exec${query === "" ? "" : `?${query}`}`;
    const ws = new WebSocket(url);

    ws.addEventListener("open", () => {
      ws.send(
        JSON.stringify({
          type: "resize",
          cols: term.cols,
          rows: term.rows,
        }),
      );
    });

    ws.addEventListener("message", (event) => {
      let payload = null;
      try {
        payload = JSON.parse(event.data);
      } catch {
        return;
      }
      if (payload.type === "stdout" || payload.type === "stderr") {
        term.write(payload.data);
      } else if (payload.type === "exit") {
        term.writeln(`\r\n[exited ${payload.code}]`);
        ws.close();
      }
    });

    const onData = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "stdin", data }));
      }
    });

    const onResize = term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols, rows }));
      }
    });

    const resizeObserver = new ResizeObserver(() => fitAddon.fit());
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      onData.dispose();
      onResize.dispose();
      ws.close();
      term.dispose();
    };
  }, [namespace, pod, container]);

  return h("div", {
    ref: containerRef,
    class: "bg-black p-2 rounded w-full",
    style: "height: 24rem;",
  });
};

export default Terminal;
