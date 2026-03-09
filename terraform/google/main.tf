resource "google_project_service" "storage" {
  service = "storage.googleapis.com"
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
