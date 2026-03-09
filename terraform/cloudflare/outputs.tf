# Used by workers/wrangler.jsonc
output "paste_kv_namespace_id" {
  description = "KV namespace ID for paste worker"
  value       = cloudflare_workers_kv_namespace.paste.id
}

output "paste_r2_bucket_name" {
  description = "R2 bucket name for paste worker"
  value       = cloudflare_r2_bucket.paste.name
}
