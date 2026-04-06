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

resource "cloudflare_zero_trust_access_application" "cortex_api_public" {
  account_id   = var.account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = "cortex-api-public.kaidotio.dev"
    },
  ]
  domain                   = "cortex-api-public.kaidotio.dev"
  name                     = "Cortex API (Public)"
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

resource "cloudflare_zero_trust_access_application" "http_kvs_public" {
  account_id   = var.account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = "http-kvs-public.kaidotio.dev"
    },
  ]
  domain                   = "http-kvs-public.kaidotio.dev"
  name                     = "HTTP KVS (Public)"
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

resource "cloudflare_zero_trust_access_application" "token_request_server_public" {
  account_id   = var.account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = "token-request-server-public.kaidotio.dev"
    },
  ]
  domain                   = "token-request-server-public.kaidotio.dev"
  name                     = "Token Request Server (Public)"
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

resource "cloudflare_zero_trust_access_application" "github_token_server_public" {
  account_id   = var.account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = "hippocampus-dev-github-token-server-public.kaidotio.dev"
    },
  ]
  domain                   = "hippocampus-dev-github-token-server-public.kaidotio.dev"
  name                     = "GitHub Token Server (Public)"
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

resource "cloudflare_zero_trust_access_application" "device_flow_bridge_public" {
  account_id   = var.account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = "hippocampus-dev-device-flow-bridge-public.kaidotio.dev"
    },
  ]
  domain                   = "hippocampus-dev-device-flow-bridge-public.kaidotio.dev"
  name                     = "Device Flow Bridge (Public)"
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

resource "cloudflare_zero_trust_access_application" "url_shortener_public" {
  account_id   = var.account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = "url-shortener-public.kaidotio.dev"
    },
  ]
  domain                   = "url-shortener-public.kaidotio.dev"
  name                     = "URL Shortener (Public)"
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

resource "cloudflare_zero_trust_access_application" "reporting_server_public" {
  account_id   = var.account_id
  allowed_idps = []
  destinations = [
    {
      type = "public"
      uri  = "reporting-server-public.kaidotio.dev"
    },
  ]
  domain                   = "reporting-server-public.kaidotio.dev"
  name                     = "Reporting Server (Public)"
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
