export type {
  AiProvider,
  ChatCompletionOptions,
  ChatCompletionResponse,
  ChatMessage,
} from "./types";

import { getCloudflareContext } from "@opennextjs/cloudflare";
import { CloudflareAiProvider } from "./cloudflare";
import type { AiProvider } from "./types";

export async function getAiProvider(): Promise<AiProvider> {
  try {
    const { env: environment } = await getCloudflareContext({ async: true });
    if (environment?.AI) {
      return new CloudflareAiProvider(environment.AI);
    }
  } catch {}

  throw new Error(
    "Unsupported platform: currently only Cloudflare Workers is supported",
  );
}
