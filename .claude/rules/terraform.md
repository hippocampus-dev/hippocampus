---
paths:
  - "terraform/**/*.tf"
---

* Organize resources by provider using modules (`google/`, `cloudflare/`, `github/`)
* Use `~> X.0` version constraints for providers
* Enable API services before creating resources with `depends_on`

## GCS Buckets

| Bucket Type | Security Settings |
|-------------|-------------------|
| Regular storage | `versioning.enabled = true`, lifecycle rules |
| Terraform state | Add `uniform_bucket_level_access = true`, `public_access_prevention = "enforced"` |

## IAM Member vs Binding

| Resource | Behavior | Use Case |
|----------|----------|----------|
| `google_*_iam_member` | Additive | WIF, shared resources |
| `google_*_iam_binding` | Authoritative | Exclusive ownership |

Use `_iam_member` for WIF to avoid removing other principals.
