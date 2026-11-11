# Used by .github/workflows/80_drive-sync.yaml

output "workload_identity_provider" {
  description = "Workload Identity Provider resource name for GitHub Actions"
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "drive_sync_service_account_email" {
  description = "Service account email for Google Drive sync"
  value       = google_service_account.drive_sync.email
}

# Used by cluster/bin/export.sh

output "cluster_autoscaler_service_account_key" {
  description = "Base64-encoded JSON key of the cluster-autoscaler service account"
  value       = google_service_account_key.cluster_autoscaler.private_key
  sensitive   = true
}
