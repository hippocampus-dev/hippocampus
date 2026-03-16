import { defineWebSocketHandler } from "h3";
import type { ServerMessage } from "@/lib/types";
import { getRunState } from "@/lib/run-state";

interface Peer {
  send(data: string): void;
}

const clients = new Set<Peer>();

export function broadcast(message: ServerMessage): void {
  const data = JSON.stringify(message);
  for (const client of clients) {
    try {
      client.send(data);
    } catch {
      clients.delete(client);
    }
  }
}

export default defineWebSocketHandler({
  open(peer) {
    clients.add(peer);
    peer.send(JSON.stringify({ type: "run_state", data: getRunState() }));
  },
  close(peer) {
    clients.delete(peer);
  },
  message(_peer, _message) {
    // Client → Server messages not used; control is via REST API
  },
});
