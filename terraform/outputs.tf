output "paste_kv_namespace_id" {
  description = "KV namespace ID for paste worker"
  value       = module.cloudflare.paste_kv_namespace_id
}

output "paste_r2_bucket_name" {
  description = "R2 bucket name for paste worker"
  value       = module.cloudflare.paste_r2_bucket_name
}
