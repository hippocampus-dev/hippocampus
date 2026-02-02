resource "cloudflare_zone" "main" {
  account = {
    id = var.account_id
  }
  name = "kaidotio.dev"
}

resource "cloudflare_ruleset" "rate_limiting" {
  zone_id     = cloudflare_zone.main.id
  name        = "Rate Limiting"
  description = "Rate limiting rules"
  kind        = "zone"
  phase       = "http_ratelimit"

  rules = [{
    ref         = "default_rate_limit"
    expression  = "true"
    description = "Default rate limit"
    action      = "block"
    enabled     = true

    ratelimit = {
      characteristics     = ["cf.colo.id", "ip.src"]
      period              = 10
      requests_per_period = 20
      mitigation_timeout  = 10
    }
  }]
}

resource "cloudflare_ruleset" "firewall_custom" {
  zone_id     = cloudflare_zone.main.id
  name        = "Custom Firewall Rules"
  description = "Custom firewall rules"
  kind        = "zone"
  phase       = "http_request_firewall_custom"

  rules = [{
    ref         = "block_non_japan"
    expression  = "(ip.geoip.country ne \"JP\") and not cf.client.bot"
    description = "Block traffic from outside Japan (except verified bots)"
    action      = "block"
    enabled     = true
  }]
}

resource "cloudflare_access_rule" "whitelist_private_network" {
  zone_id = cloudflare_zone.main.id
  mode    = "whitelist"
  notes   = "Private Network IPv6"
  configuration = {
    target = "ip_range"
    value  = "240d:f:431:9a00::/64"
  }
}

resource "cloudflare_ruleset" "ddos_l7" {
  zone_id     = cloudflare_zone.main.id
  name        = "HTTP DDoS Attack Protection"
  description = "DDoS protection settings"
  kind        = "zone"
  phase       = "ddos_l7"

  rules = [{
    ref         = "ddos_default"
    expression  = "true"
    description = "Default DDoS protection"
    action      = "execute"
    enabled     = true

    action_parameters = {
      id = "4d21379b4f9f4bb088e0729962c8b3cf"
      overrides = {
        sensitivity_level = "default"
      }
    }
  }]
}

resource "cloudflare_ruleset" "firewall_managed" {
  zone_id     = cloudflare_zone.main.id
  name        = "Cloudflare Free Managed Ruleset"
  description = "Execute Free managed WAF rules"
  kind        = "zone"
  phase       = "http_request_firewall_managed"

  rules = [{
    ref         = "execute_free_managed"
    expression  = "true"
    description = "Execute Free managed ruleset"
    action      = "execute"
    enabled     = true

    action_parameters = {
      id = "77454fe2d30c4220b5701f6fdfb893ba"
    }
  }]
}

resource "cloudflare_ruleset" "cache_rules" {
  zone_id     = cloudflare_zone.main.id
  name        = "Cache Rules"
  description = "Custom cache behavior"
  kind        = "zone"
  phase       = "http_request_cache_settings"

  rules = [{
    ref         = "bypass_api_cache"
    expression  = "starts_with(http.request.uri.path, \"/api/\")"
    description = "Bypass cache for API endpoints"
    action      = "set_cache_settings"
    enabled     = true

    action_parameters = {
      cache = false
    }
  }]
}

resource "cloudflare_ruleset" "redirect_rules" {
  zone_id     = cloudflare_zone.main.id
  name        = "Redirect Rules"
  description = "URL redirect rules"
  kind        = "zone"
  phase       = "http_request_dynamic_redirect"

  rules = [{
    ref         = "non_www_to_www"
    expression  = "(http.host eq \"kaidotio.dev\")"
    description = "Redirect non-www to www"
    action      = "redirect"
    enabled     = true

    action_parameters = {
      from_value = {
        target_url = {
          expression = "concat(\"https://www.kaidotio.dev\", http.request.uri.path)"
        }
        status_code           = 301
        preserve_query_string = true
      }
    }
  }]
}

resource "cloudflare_ruleset" "response_headers_transform" {
  zone_id     = cloudflare_zone.main.id
  name        = "Response Headers Transform"
  description = "Add security headers to responses"
  kind        = "zone"
  phase       = "http_response_headers_transform"

  rules = [{
    ref         = "security_headers"
    expression  = "true"
    description = "Add security headers"
    action      = "rewrite"
    enabled     = true

    action_parameters = {
      headers = {
        "X-Frame-Options" = {
          operation = "set"
          value     = "DENY"
        }
        "X-XSS-Protection" = {
          operation = "set"
          value     = "1; mode=block"
        }
      }
    }
  }]
}

resource "cloudflare_url_normalization_settings" "main" {
  zone_id = cloudflare_zone.main.id
  scope   = "incoming"
  type    = "cloudflare"
}

resource "cloudflare_web_analytics_site" "main" {
  account_id   = var.account_id
  zone_tag     = cloudflare_zone.main.id
  auto_install = true
}

resource "cloudflare_zone_dnssec" "main" {
  zone_id = cloudflare_zone.main.id
}

resource "cloudflare_tiered_cache" "main" {
  zone_id = cloudflare_zone.main.id
  value   = "on"
}

resource "cloudflare_bot_management" "main" {
  zone_id               = cloudflare_zone.main.id
  ai_bots_protection    = "block"
  crawler_protection    = "enabled"
  enable_js             = true
  fight_mode            = true
  is_robots_txt_managed = true
}

resource "cloudflare_zone_setting" "cache_level" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "cache_level"
  value      = "aggressive"
}

resource "cloudflare_zone_setting" "always_use_https" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "always_use_https"
  value      = "on"
}

resource "cloudflare_zone_setting" "min_tls_version" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "min_tls_version"
  value      = "1.2"
}

resource "cloudflare_zone_setting" "tls_1_3" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "tls_1_3"
  value      = "on"
}

resource "cloudflare_zone_setting" "http3" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "http3"
  value      = "on"
}

resource "cloudflare_zone_setting" "zero_rtt" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "0rtt"
  value      = "off"
}

resource "cloudflare_zone_setting" "brotli" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "brotli"
  value      = "on"
}

resource "cloudflare_zone_setting" "challenge_ttl" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "challenge_ttl"
  value      = "1800"
}

resource "cloudflare_zone_setting" "ip_geolocation" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "ip_geolocation"
  value      = "on"
}

resource "cloudflare_zone_setting" "websockets" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "websockets"
  value      = "on"
}

resource "cloudflare_zone_setting" "always_online" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "always_online"
  value      = "on"
}

resource "cloudflare_zone_setting" "opportunistic_onion" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "opportunistic_onion"
  value      = "on"
}

# Settings that can be overridden by Configuration Rules

resource "cloudflare_zone_setting" "security_level" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "security_level"
  value      = "medium"
}

resource "cloudflare_zone_setting" "browser_check" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "browser_check"
  value      = "on"
}

resource "cloudflare_zone_setting" "hotlink_protection" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "hotlink_protection"
  value      = "on"
}

resource "cloudflare_zone_setting" "ssl" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "ssl"
  value      = "strict"
}

resource "cloudflare_zone_setting" "security_header" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "security_header"
  value = {
    strict_transport_security = {
      enabled            = true
      include_subdomains = true
      max_age            = 31536000
      nosniff            = true
      preload            = true
    }
  }
}

resource "cloudflare_zone_setting" "automatic_https_rewrites" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "automatic_https_rewrites"
  value      = "on"
}

resource "cloudflare_zone_setting" "early_hints" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "early_hints"
  value      = "on"
}

resource "cloudflare_zone_setting" "opportunistic_encryption" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "opportunistic_encryption"
  value      = "on"
}

resource "cloudflare_zone_setting" "server_side_excludes" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "server_side_exclude"
  value      = "on"
}

resource "cloudflare_zone_setting" "rocket_loader" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "rocket_loader"
  value      = "off"
}

resource "cloudflare_zone_setting" "email_obfuscation" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "email_obfuscation"
  value      = "on"
}

resource "cloudflare_zone_setting" "minify" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "minify"
  value = {
    css  = "off"
    html = "off"
    js   = "off"
  }
}

resource "cloudflare_ruleset" "config_rules_pages" {
  zone_id     = cloudflare_zone.main.id
  name        = "Configuration Rules"
  description = "Per-host configuration overrides"
  kind        = "zone"
  phase       = "http_config_settings"

  rules = [{
    ref         = "pages_config"
    expression  = "http.host eq \"${var.pages_domain}\""
    description = "Pages: enable HTML minify, override other settings explicitly"
    action      = "set_config"
    enabled     = true

    action_parameters = {
      security_level           = "medium"
      browser_integrity_check  = true
      hotlink_protection       = true
      ssl                      = "strict"
      automatic_https_rewrites = true
      early_hints              = true
      opportunistic_encryption = true
      server_side_excludes     = true
      rocket_loader            = false
      email_obfuscation        = true
      autominify = {
        css  = false
        html = true
        js   = false
      }
    }
  }]
}
