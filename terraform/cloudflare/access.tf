resource "cloudflare_zero_trust_access_application" "pages" {
  account_id   = var.account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = var.pages_domain
    },
  ]
  domain                   = var.pages_domain
  name                     = "Pages (Public)"
  options_preflight_bypass = false
  policies = [
    {
      decision = "bypass"
      exclude  = []
      include = [
        {
          everyone = {}
        },
      ]
      name       = "Public Access"
      precedence = 1
      require    = []
    },
  ]
  session_duration = "24h"
  type             = "self_hosted"
}

resource "cloudflare_zero_trust_access_application" "local" {
  account_id   = var.account_id
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
      exclude  = []
      include = [
        {
          email = {
            email = "kaidotio@gmail.com"
          }
        },
      ]
      name       = "Admin"
      precedence = 1
      require    = []
    },
    {
      decision = "bypass"
      exclude  = []
      include = [
        {
          ip = {
            ip = "240d:f:431:9a00:ea65:38ff:fe93:93dd/128"
          }
        },
      ]
      name       = "Private Network"
      precedence = 2
      require    = []
    },
  ]
  session_duration = "24h"
  type             = "self_hosted"
}
