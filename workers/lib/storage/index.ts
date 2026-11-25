export type {
  ExplanationRepository,
  PasteCreateInput,
  PasteMetadata,
  PasteRepository,
  PasteWithContent,
} from "./types";

import { getCloudflareContext } from "@opennextjs/cloudflare";
import {
  CloudflareExplanationRepository,
  CloudflarePasteRepository,
} from "./cloudflare";
import type { ExplanationRepository, PasteRepository } from "./types";

export async function getPasteRepository(): Promise<PasteRepository> {
  try {
    const { env: environment } = await getCloudflareContext({ async: true });
    if (environment?.PASTE_KV && environment?.PASTE_BUCKET) {
      return new CloudflarePasteRepository(
        environment.PASTE_KV,
        environment.PASTE_BUCKET,
      );
    }
  } catch {}

  throw new Error(
    "Unsupported platform: currently only Cloudflare Workers is supported",
  );
}

export async function getExplanationRepository(): Promise<ExplanationRepository> {
  try {
    const { env: environment } = await getCloudflareContext({ async: true });
    if (environment?.PASTE_KV) {
      return new CloudflareExplanationRepository(environment.PASTE_KV);
    }
  } catch {}

  throw new Error(
    "Unsupported platform: currently only Cloudflare Workers is supported",
  );
}
