import type { Buffer } from "@/lib/buffer";

declare global {
  interface CloudflareEnv {
    PASTE_KV: KVNamespace;
    PASTE_BUCKET: R2Bucket;
    AI: Ai;
    LOADER: WorkerLoader;
    BUFFER: DurableObjectNamespace<Buffer>;
  }
}

export {};
