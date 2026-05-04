resource "cloudflare_pages_project" "main" {
  account_id        = var.account_id
  name              = "kaidotio-hippocampus"
  production_branch = "main"

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
      deployments_enabled           = true
      production_deployment_enabled = true
      preview_deployment_setting    = "none"
    }
  }

  deployment_configs = {
    preview = {}
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

resource "cloudflare_pages_domain" "www" {
  account_id   = var.account_id
  project_name = cloudflare_pages_project.main.name
  name         = var.pages_domain
}
