export type {
    AiProvider,
    ChatMessage,
    ChatCompletionOptions,
    ChatCompletionResponse,
} from "./types";

import type {AiProvider} from "./types";
import {getCloudflareContext} from "@opennextjs/cloudflare";
import {CloudflareAiProvider} from "./cloudflare";

export async function getAiProvider(): Promise<AiProvider> {
    try {
        const {env} = await getCloudflareContext({async: true});
        if (env?.AI) {
            return new CloudflareAiProvider(env.AI);
        }
    } catch {
    }

    throw new Error("Unsupported platform: currently only Cloudflare Workers is supported");
}
