export type { CodeRunner, RunOutput, RunResult } from "./types";

import { getCloudflareContext } from "@opennextjs/cloudflare";
import {
  CloudflareCodeRunner,
  type DynamicWorkerTailFactory,
} from "./cloudflare";
import type { CodeRunner } from "./types";

type CtxWithExports = ExecutionContext & {
  exports?: { DynamicWorkerTail?: DynamicWorkerTailFactory };
};

export async function getCodeRunner(): Promise<CodeRunner> {
  try {
    const { env, ctx } = await getCloudflareContext({ async: true });
    const tailFactory = (ctx as CtxWithExports)?.exports?.DynamicWorkerTail;
    if (env?.LOADER && env?.BUFFER && tailFactory) {
      return new CloudflareCodeRunner(env.LOADER, env.BUFFER, tailFactory);
    }
  } catch {}

  throw new Error(
    "Unsupported platform: currently only Cloudflare Workers is supported",
  );
}
