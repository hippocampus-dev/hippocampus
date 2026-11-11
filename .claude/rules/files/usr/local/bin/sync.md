---
paths:
  - "files/usr/local/bin/sync.sh"
---

* Put a new sync target in `gs://kaidotio-sync` rather than under `/mnt/gdrive` - the bucket's noncurrent-version retention is declared in `terraform/google/main.tf` (`versioning` and its `lifecycle_rule`), while `files/etc/systemd/user/rclone-gdrive.service` mounts `gdrive:` without a crypt remote; Cloud Storage charges data transfer only on download, so a target this script uploads daily costs storage alone
