resource "cloudflare_workers_kv_namespace" "rate_limit" {
  account_id = var.account_id
  title      = "rate-limit"
}

resource "cloudflare_workers_kv_namespace" "paste" {
  account_id = var.account_id
  title      = "paste"
}

resource "cloudflare_r2_bucket" "paste" {
  account_id = var.account_id
  name       = "paste"
}
