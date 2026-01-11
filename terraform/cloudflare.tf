resource "cloudflare_zone" "main" {
  account = {
    id = var.cloudflare_account_id
  }
  name = "kaidotio.dev"
}

resource "cloudflare_email_routing_rule" "primary" {
  zone_id = cloudflare_zone.main.id
  actions = [{
    type  = "forward"
    value = ["kaidotio@gmail.com"]
  }]
  matchers = [{
    type  = "literal"
    field = "to"
    value = "0@kaidotio.dev"
  }]
  enabled = true
}

resource "cloudflare_zero_trust_access_application" "local" {
  account_id   = var.cloudflare_account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = "*.kaidotio.dev"
    },
  ]
  domain                   = "*.kaidotio.dev"
  name                     = "Local"
  options_preflight_bypass = false
  policies = [
    {
      decision = "allow"
      exclude = [
      ]
      include = [
        {
          email = {
            email = "kaidotio@gmail.com"
          }
        },
      ]
      name       = "Admin"
      precedence = 1
      require = [
      ]
    },
    {
      decision = "bypass"
      exclude = [
      ]
      include = [
        {
          ip = {
            ip = "240d:f:431:9a00:ea65:38ff:fe93:93dd/128"
          }
        },
      ]
      name       = "Private Network"
      precedence = 2
      require = [
      ]
    },
  ]
  session_duration = "24h"
  type             = "self_hosted"
}

resource "cloudflare_workers_kv_namespace" "rate_limit" {
  account_id = var.cloudflare_account_id
  title      = "rate-limit"
}

resource "cloudflare_pages_project" "main" {
  account_id = var.cloudflare_account_id
  name       = "kaidotio-hippocampus"

  build_config = {
    root_dir        = "pages"
    build_command   = "find . -type l | while read f; do cp --remove-destination $(readlink -f $f) $f; done && npm ci"
    destination_dir = "."
  }

  source = {
    type = "github"
    config = {
      owner                         = "kaidotio"
      repo_name                     = "hippocampus"
      production_branch             = "main"
      deployments_enabled           = true
      production_deployment_enabled = true
      preview_deployment_setting    = "none"
    }
  }

  deployment_configs = {
    production = {
      compatibility_date = "2025-12-25"
      kv_namespaces = {
        RATE_LIMIT_KV = {
          namespace_id = cloudflare_workers_kv_namespace.rate_limit.id
        }
      }
    }
  }
}
