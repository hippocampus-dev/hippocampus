# Used by .github/workflows/80_drive-sync.yaml

output "workload_identity_provider" {
  description = "Workload Identity Provider resource name for GitHub Actions"
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "drive_sync_service_account_email" {
  description = "Service account email for Google Drive sync"
  value       = google_service_account.drive_sync.email
}
