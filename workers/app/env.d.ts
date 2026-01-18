declare global {
  interface CloudflareEnv {
    PASTE_KV: KVNamespace;
    PASTE_BUCKET: R2Bucket;
    AI: Ai;
  }
}

export {};
