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
    const { env: environment, ctx: context } = await getCloudflareContext({
      async: true,
    });
    const tailFactory = (context as CtxWithExports)?.exports?.DynamicWorkerTail;
    if (environment?.LOADER && environment?.BUFFER && tailFactory) {
      return new CloudflareCodeRunner(
        environment.LOADER,
        environment.BUFFER,
        tailFactory,
      );
    }
  } catch {}

  throw new Error(
    "Unsupported platform: currently only Cloudflare Workers is supported",
  );
}
