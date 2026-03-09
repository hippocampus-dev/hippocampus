import { nanoid } from "nanoid";
import type {
  PasteRepository,
  PasteMetadata,
  PasteWithContent,
  PasteCreateInput,
  ExplanationRepository,
} from "./types";

const MIN_TTL_SECONDS = 60;
const ID_PATTERN = /^[A-Za-z0-9_-]{10}$/;

function isValidId(id: string): boolean {
  return ID_PATTERN.test(id);
}

export class CloudflarePasteRepository implements PasteRepository {
  private kv: KVNamespace;
  private bucket: R2Bucket;

  constructor(kv: KVNamespace, bucket: R2Bucket) {
    this.kv = kv;
    this.bucket = bucket;
  }

  async create(input: PasteCreateInput): Promise<PasteMetadata> {
    const id = nanoid(10);
    const now = new Date();

    const metadata: PasteMetadata = {
      id,
      language: input.language || "text",
      title: input.title || "Untitled",
      createdAt: now.toISOString(),
      expiresAt: input.expiresAt?.toISOString() ?? null,
      size: new TextEncoder().encode(input.content).length,
    };

    await this.bucket.put(id, input.content, {
      customMetadata: {
        language: metadata.language,
        title: metadata.title,
      },
    });

    const kvOptions: KVNamespacePutOptions = {};
    if (input.expiresAt) {
      const ttl = Math.floor(
        (input.expiresAt.getTime() - now.getTime()) / 1000
      );
      if (ttl >= MIN_TTL_SECONDS) {
        kvOptions.expirationTtl = ttl;
      }
    }
    await this.kv.put(`paste:${id}`, JSON.stringify(metadata), kvOptions);

    return metadata;
  }

  async findById(id: string): Promise<PasteWithContent | null> {
    if (!isValidId(id)) {
      return null;
    }

    const stored = await this.kv.get(`paste:${id}`);
    if (!stored) {
      return null;
    }
    const metadata = JSON.parse(stored) as PasteMetadata;

    const object = await this.bucket.get(id);
    if (!object) {
      return null;
    }
    const content = await object.text();

    return { ...metadata, content };
  }

  async deleteById(id: string): Promise<void> {
    if (!isValidId(id)) {
      return;
    }

    await Promise.all([
      this.kv.delete(`paste:${id}`),
      this.kv.delete(`explanation:${id}`),
      this.bucket.delete(id),
    ]);
  }
}

export class CloudflareExplanationRepository implements ExplanationRepository {
  private kv: KVNamespace;

  constructor(kv: KVNamespace) {
    this.kv = kv;
  }

  async get(pasteId: string): Promise<string | null> {
    if (!isValidId(pasteId)) {
      return null;
    }

    return this.kv.get(`explanation:${pasteId}`);
  }

  async set(
    pasteId: string,
    explanation: string,
    ttlSeconds?: number
  ): Promise<void> {
    if (!isValidId(pasteId)) {
      return;
    }

    const options: KVNamespacePutOptions = {};
    if (ttlSeconds !== undefined && ttlSeconds >= MIN_TTL_SECONDS) {
      options.expirationTtl = ttlSeconds;
    }
    await this.kv.put(`explanation:${pasteId}`, explanation, options);
  }
}
