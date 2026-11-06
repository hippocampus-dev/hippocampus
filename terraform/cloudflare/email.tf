resource "cloudflare_email_routing_rule" "notification" {
  zone_id = cloudflare_zone.main.id
  actions = [{
    type  = "forward"
    value = ["kaidotio@gmail.com"]
  }]
  matchers = [{
    type  = "literal"
    field = "to"
    value = "notification@kaidotio.dev"
  }]
  enabled = true
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
