resource "cloudflare_dns_record" "localhost" {
  zone_id = cloudflare_zone.main.id
  name    = "localhost"
  type    = "A"
  content = "127.0.0.1"
  ttl     = 86400
  proxied = false
}

resource "cloudflare_dns_record" "dmarc" {
  zone_id = cloudflare_zone.main.id
  name    = "_dmarc"
  type    = "TXT"
  content = "v=DMARC1; p=reject; sp=reject; adkim=s; aspf=s; rua=mailto:dmarc@kaidotio.dev"
  ttl     = 86400
}
