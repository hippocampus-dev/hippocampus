import type { Buffer } from "@/lib/buffer";
import type { CodeRunner, RunOutput, RunResult } from "./types";

// @ts-expect-error -- raw text import for Dynamic Worker module
import runnerTemplate from "./templates/runner.js?raw";

const WALL_TIMEOUT_MS = 5000;
const LOG_POLL_TIMEOUT_MS = 1000;
const COMPATIBILITY_DATE = "2026-03-01";
const RUNNABLE_LANGUAGES = new Set(["javascript", "python"]);

export type DynamicWorkerTailFactory = (opts: {
  props: { bufferName: string };
}) => Fetcher;

export class CloudflareCodeRunner implements CodeRunner {
  private loader: WorkerLoader;
  private buffer: DurableObjectNamespace<Buffer>;
  private tailFactory: DynamicWorkerTailFactory;

  constructor(
    loader: WorkerLoader,
    buffer: DurableObjectNamespace<Buffer>,
    tailFactory: DynamicWorkerTailFactory,
  ) {
    this.loader = loader;
    this.buffer = buffer;
    this.tailFactory = tailFactory;
  }

  async run(code: string, language: string): Promise<RunResult> {
    if (!RUNNABLE_LANGUAGES.has(language)) {
      return {
        output: [],
        error: `Unsupported language: ${language}`,
        wallTimeMs: 0,
        timedOut: false,
      };
    }

    const sessionId = crypto.randomUUID();
    const start = performance.now();

    const modules: Record<string, WorkerLoaderModule | string> = {
      "runner.js": runnerTemplate,
      "user.js": {
        js:
          language === "python"
            ? `import * as py from "./user.py"; export default py.default;`
            : code,
      },
    };
    if (language === "python") {
      modules["user.py"] = { py: code };
    }

    const bufferId = this.buffer.idFromName(sessionId);
    const bufferStub = this.buffer.get(bufferId);

    try {
      const worker = this.loader.get(sessionId, () => ({
        compatibilityDate: COMPATIBILITY_DATE,
        mainModule: "runner.js",
        modules,
        env: {},
        globalOutbound: null,
        tails: [
          this.tailFactory({
            props: { bufferName: sessionId },
          }),
        ],
      }));

      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), WALL_TIMEOUT_MS);

      const entrypoint = worker.getEntrypoint() as unknown as {
        run(): Promise<void>;
      };

      try {
        await Promise.race([
          entrypoint.run(),
          new Promise((_, reject) => {
            controller.signal.addEventListener("abort", () => {
              reject(new Error("Execution timed out"));
            });
          }),
        ]);
      } finally {
        clearTimeout(timeout);
      }

      const output = (await bufferStub.read(
        LOG_POLL_TIMEOUT_MS,
      )) as RunOutput[];
      const wallTimeMs = Math.round(performance.now() - start);

      return {
        output,
        error: null,
        wallTimeMs,
        timedOut: false,
      };
    } catch (error) {
      const wallTimeMs = Math.round(performance.now() - start);
      const timedOut =
        error instanceof Error && error.message === "Execution timed out";

      let output: RunOutput[] = [];
      try {
        output = (await bufferStub.read(LOG_POLL_TIMEOUT_MS)) as RunOutput[];
      } catch {}

      return {
        output,
        error: timedOut ? "Execution timed out (5s limit)" : String(error),
        wallTimeMs,
        timedOut,
      };
    }
  }
}
