export type { CodeRunner, RunOutput, RunResult } from "./types";

import { getCloudflareContext } from "@opennextjs/cloudflare";
import { CloudflareCodeRunner } from "./cloudflare";
import type { CodeRunner } from "./types";

export async function getCodeRunner(): Promise<CodeRunner> {
  try {
    const { env } = await getCloudflareContext({ async: true });
    if (env?.LOADER && env?.BUFFER) {
      return new CloudflareCodeRunner(env.LOADER, env.BUFFER);
    }
  } catch {}

  throw new Error(
    "Unsupported platform: currently only Cloudflare Workers is supported",
  );
}
