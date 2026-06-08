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

## Cloudflare (Pro Plan)

The Cloudflare account uses the **Pro plan**.

| Feature | Details |
|---------|---------|
| Rate Limiting | `period` = 10, `mitigation_timeout` = 10, `action` = block only |
| Custom Firewall Rules | Max 20 rules |
| Managed Rulesets | DDoS L7 (`4d21379b4f9f4bb088e0729962c8b3cf`), Cloudflare Managed (`efb7b8c949ac4650a09736fc376e9aee`), OWASP Core (`4814384a9e5d4991b9815dcfc25d2f1f`) |
| URL Normalization | Use `cloudflare_url_normalization_settings` resource (not ruleset) |
| Bot Management | `fight_mode`, `enable_js`, `ai_bots_protection`, `sbfm_definitely_automated`, `sbfm_verified_bots` |
| Super Bot Fight Mode Skip | Use `action = "skip"` with `phases = ["http_request_sbfm"]` in `firewall_custom` for `-public.kaidotio.dev` hosts |
| Image Optimization | `polish = "lossy"`, `webp = "on"` |
| `matches` operator | Not available (requires Business plan) |

### Firewall Expression Fields

| Field | Plan Required | Use Case |
|-------|---------------|----------|
| `cf.client.bot` | Free | Detect known good bots |
| `cf.bot_management.verified_bot` | Enterprise | Verify bot identity (Enterprise Bot Management) |

Use `cf.client.bot` in firewall expressions to allow verified bots.

## Reference

If adding new Terraform resources:
  Read: `.claude/reference/terraform/adding-resources.md`
