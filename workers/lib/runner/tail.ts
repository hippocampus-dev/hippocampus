import { WorkerEntrypoint } from "cloudflare:workers";
import type { RunOutput } from "./types";

type Props = { bufferName: string };

export class DynamicWorkerTail extends WorkerEntrypoint<CloudflareEnv, Props> {
  async tail(events: TraceItem[]): Promise<void> {
    const { bufferName } = this.ctx.props;
    const bufferId = this.env.BUFFER.idFromName(bufferName);
    const bufferStub = this.env.BUFFER.get(bufferId);

    const items: RunOutput[] = [];

    for (const event of events) {
      for (const log of event.logs) {
        const level =
          log.level === "warn"
            ? "warn"
            : log.level === "error"
              ? "error"
              : "log";
        items.push({
          level,
          message: String(log.message),
        });
      }

      for (const exception of event.exceptions) {
        items.push({
          level: "exception",
          message: `${exception.name}: ${exception.message}`,
        });
      }
    }

    await bufferStub.write(items);
  }
}
