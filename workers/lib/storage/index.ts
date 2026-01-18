export type {
    PasteRepository,
    PasteMetadata,
    PasteWithContent,
    PasteCreateInput,
    ExplanationRepository,
} from "./types";

import type {ExplanationRepository, PasteRepository} from "./types";
import {getCloudflareContext} from "@opennextjs/cloudflare";
import {CloudflareExplanationRepository, CloudflarePasteRepository,} from "./cloudflare";

export async function getPasteRepository(): Promise<PasteRepository> {
    try {
        const {env} = await getCloudflareContext({async: true});
        if (env?.PASTE_KV && env?.PASTE_BUCKET) {
            return new CloudflarePasteRepository(env.PASTE_KV, env.PASTE_BUCKET);
        }
    } catch {
    }

    throw new Error("Unsupported platform: currently only Cloudflare Workers is supported");
}

export async function getExplanationRepository(): Promise<ExplanationRepository> {
    try {
        const {env} = await getCloudflareContext({async: true});
        if (env?.PASTE_KV) {
            return new CloudflareExplanationRepository(env.PASTE_KV);
        }
    } catch {
    }

    throw new Error("Unsupported platform: currently only Cloudflare Workers is supported");
}
