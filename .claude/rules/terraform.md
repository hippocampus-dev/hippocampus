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

## Cloudflare (Free Plan)

The Cloudflare account uses the **Free plan**. Avoid features requiring paid plans.

| Feature | Free Plan |
|---------|-----------|
| Rate Limiting | `period` = 10, `mitigation_timeout` = 10, `action` = block only |
| Custom Firewall Rules | Max 5 rules |
| Managed Rulesets | DDoS L7 (`4d21379b4f9f4bb088e0729962c8b3cf`), Free Managed (`77454fe2d30c4220b5701f6fdfb893ba`) |
| URL Normalization | Use `cloudflare_url_normalization_settings` resource (not ruleset) |
| Bot Management | `fight_mode`, `enable_js`, `ai_bots_protection` |
| WAF (OWASP, Exposed Credentials) | Not available |

### Firewall Expression Fields

| Field | Plan Required | Use Case |
|-------|---------------|----------|
| `cf.client.bot` | Free | Detect known good bots |
| `cf.bot_management.verified_bot` | Enterprise | Verify bot identity (Enterprise Bot Management) |

Use `cf.client.bot` in firewall expressions to allow verified bots on Free plan.

## Reference

If adding new Terraform resources:
  Read: `.claude/rules/.reference/terraform/adding-resources.md`
