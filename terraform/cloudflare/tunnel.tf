resource "cloudflare_zero_trust_tunnel_cloudflared_config" "main" {
  account_id = var.account_id
  tunnel_id  = "f529d3ba-eff9-4765-b1bc-dd7efca3f61f"
  config = {
    ingress = [
      {
        hostname = "*.kaidotio.dev"
        service  = "http://127.0.0.1:10081"
      },
      {
        service = "http_status:404"
      },
    ]
  }
}
