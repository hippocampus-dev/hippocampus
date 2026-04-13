import type { CodeRunner, RunOutput, RunResult } from "./types";

// @ts-expect-error -- raw text import for Dynamic Worker module
import runnerTemplate from "./templates/runner.js?raw";

const WALL_TIMEOUT_MS = 5000;
const LOG_POLL_TIMEOUT_MS = 1000;
const COMPATIBILITY_DATE = "2026-03-01";
const RUNNABLE_LANGUAGES = new Set(["javascript", "python"]);

function formatTailLogs(
  logs: { level: string; message: unknown }[],
  exceptions: { name: string; message: string }[],
): RunOutput[] {
  const output: RunOutput[] = [];

  for (const log of logs) {
    const level = log.level === "warn" ? "warn" : log.level === "error" ? "error" : "log";
    output.push({
      level: level as RunOutput["level"],
      message: String(log.message),
    });
  }

  for (const exception of exceptions) {
    output.push({
      level: "exception",
      message: `${exception.name}: ${exception.message}`,
    });
  }

  return output;
}

export class CloudflareCodeRunner implements CodeRunner {
  private loader: WorkerLoader;
  private buffer: DurableObjectNamespace;

  constructor(loader: WorkerLoader, buffer: DurableObjectNamespace) {
    this.loader = loader;
    this.buffer = buffer;
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

    const modules =
      language === "python"
        ? {
            "runner.js": runnerTemplate,
            "user.js": {
              js: `import * as py from "./user.py"; export default py.default;`,
            },
            "user.py": { py: code },
          }
        : {
            "runner.js": runnerTemplate,
            "user.js": { js: code },
          };

    const bufferId = this.buffer.idFromName(sessionId);
    const bufferStub = this.buffer.get(bufferId);

    try {
      const worker = this.loader.load({
        compatibilityDate: COMPATIBILITY_DATE,
        mainModule: "runner.js",
        modules,
        env: {},
        globalOutbound: null,
        tails: [
          {
            consumer: {
              async tail(events: TailEvent[]) {
                for (const event of events) {
                  const items = formatTailLogs(
                    event.logs ?? [],
                    event.exceptions ?? [],
                  );
                  await bufferStub.write(items);
                }
              },
            },
          },
        ],
      });

      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), WALL_TIMEOUT_MS);

      try {
        await Promise.race([
          worker.getEntrypoint().run(),
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

      const output = (await bufferStub
        .read(LOG_POLL_TIMEOUT_MS)
        .catch(() => [])) as RunOutput[];

      return {
        output,
        error: timedOut ? "Execution timed out (5s limit)" : String(error),
        wallTimeMs,
        timedOut,
      };
    }
  }
}
