resource "cloudflare_zone" "main" {
  account = {
    id = var.account_id
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

resource "cloudflare_email_routing_rule" "ai" {
  zone_id = cloudflare_zone.main.id
  actions = [{
    type  = "worker"
    value = ["email-worker"]
  }]
  matchers = [{
    type  = "literal"
    field = "to"
    value = "ai@kaidotio.dev"
  }]
  enabled = true
}

resource "cloudflare_ruleset" "rate_limiting" {
  zone_id     = cloudflare_zone.main.id
  name        = "Rate limiting"
  description = "Zone rate limiting rules"
  kind        = "zone"
  phase       = "http_ratelimit"

  rules = [{
    ref         = "rate_limit_by_ip"
    description = "Rate limit requests by IP"
    expression  = "true"
    action      = "managed_challenge"
    enabled     = true

    ratelimit = {
      characteristics     = ["ip.src"]
      period              = 10
      requests_per_period = 100
      mitigation_timeout  = 60
    }
  }]
}

resource "cloudflare_ruleset" "ddos_l7" {
  zone_id     = cloudflare_zone.main.id
  name        = "HTTP DDoS Attack Protection"
  description = "Explicit default DDoS settings"
  kind        = "zone"
  phase       = "ddos_l7"

  rules = [{
    ref         = "ddos_default"
    description = "Use default DDoS protection"
    expression  = "true"
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

resource "cloudflare_bot_management" "main" {
  zone_id    = cloudflare_zone.main.id
  fight_mode = true
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

resource "cloudflare_zone_setting" "http2" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "http2"
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

resource "cloudflare_zone_setting" "sort_query_string_for_cache" {
  zone_id    = cloudflare_zone.main.id
  setting_id = "sort_query_string_for_cache"
  value      = "off"
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
