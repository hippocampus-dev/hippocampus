output "paste_kv_namespace_id" {
  description = "KV namespace ID for paste worker"
  value       = module.cloudflare.paste_kv_namespace_id
}

output "paste_r2_bucket_name" {
  description = "R2 bucket name for paste worker"
  value       = module.cloudflare.paste_r2_bucket_name
}

output "workload_identity_provider" {
  description = "Workload Identity Provider resource name for GitHub Actions"
  value       = module.google.workload_identity_provider
}

output "drive_sync_service_account_email" {
  description = "Service account email for Google Drive sync"
  value       = module.google.drive_sync_service_account_email
}
