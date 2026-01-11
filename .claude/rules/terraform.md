---
paths:
  - "terraform/**/*.tf"
---

* Organize resources by provider: `google.tf`, `cloudflare.tf`, `github.tf`
* Use `~> X.0` version constraints for providers (allows patch updates within major version)
* Mark sensitive variables with `sensitive = true`
* Enable API services before creating resources with `depends_on`

## GCS Buckets

| Bucket Type | Security Settings |
|-------------|-------------------|
| Regular storage | `versioning.enabled = true`, lifecycle rules |
| Terraform state | Add `uniform_bucket_level_access = true`, `public_access_prevention = "enforced"` |

State buckets require stricter security to protect potentially sensitive infrastructure data.

## Workload Identity Federation (GitHub Actions)

| Resource | Purpose |
|----------|---------|
| `google_iam_workload_identity_pool` | Identity pool for external identities |
| `google_iam_workload_identity_pool_provider` | OIDC provider configuration |
| `google_service_account_iam_member` | Grant WIF access to service account |

### IAM Member vs Binding

| Resource | Behavior | Use Case |
|----------|----------|----------|
| `google_*_iam_member` | Additive (preserves existing) | WIF, shared resources |
| `google_*_iam_binding` | Authoritative (replaces all) | Exclusive ownership |

Use `_iam_member` for WIF to avoid removing other principals from the service account.

### Attribute Mapping

Required attributes for GitHub Actions:

```hcl
attribute_mapping = {
  "google.subject"             = "assertion.sub"
  "attribute.actor"            = "assertion.actor"
  "attribute.repository"       = "assertion.repository"
  "attribute.repository_owner" = "assertion.repository_owner"
}
```

### Principal Format

```
principalSet://iam.googleapis.com/${pool.name}/attribute.repository/{owner}/{repo}
```

## File Structure

| File | Purpose |
|------|---------|
| `versions.tf` | Terraform and provider versions, backend |
| `providers.tf` | Provider configurations |
| `variables.tf` | Input variable definitions |
| `{provider}.tf` | Resources for each provider |
