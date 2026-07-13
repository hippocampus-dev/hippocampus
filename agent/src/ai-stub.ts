// See: https://developers.cloudflare.com/workers/wrangler/configuration/#bundling-issues
export function jsonSchema(): never {
  throw new Error(
    "ai package not bundled: jsonSchema() should not be reachable",
  );
}
