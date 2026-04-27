resource "google_project_service" "storage" {
  service = "storage.googleapis.com"
}

resource "google_project_service" "iam_credentials" {
  service = "iamcredentials.googleapis.com"
}

resource "google_project_service" "drive" {
  service = "drive.googleapis.com"
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github"
  display_name              = "GitHub Actions"

  depends_on = [google_project_service.iam_credentials]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "GitHub Actions OIDC"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }

  attribute_condition = "attribute.repository == \"hippocampus-dev/hippocampus\" && attribute.ref == \"refs/heads/main\""

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account" "drive_sync" {
  account_id   = "drive-sync"
  display_name = "Google Drive Sync"

  depends_on = [google_project_service.iam_credentials, google_project_service.drive]
}

resource "google_service_account_iam_member" "drive_sync_workload_identity" {
  service_account_id = google_service_account.drive_sync.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/hippocampus-dev/hippocampus"
}

resource "google_storage_bucket" "sync" {
  name     = "kaidotio-sync"
  location = "ASIA-NORTHEAST1"

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      num_newer_versions = 3
      with_state         = "ARCHIVED"
    }
    action {
      type = "Delete"
    }
  }

  lifecycle_rule {
    condition {
      days_since_noncurrent_time = 7
      with_state                 = "ANY"
    }
    action {
      type = "Delete"
    }
  }

  depends_on = [google_project_service.storage]
}
