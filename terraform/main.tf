module "google" {
  source = "./google"
}

module "cloudflare" {
  source     = "./cloudflare"
  account_id = var.cloudflare_account_id
}

module "github" {
  source = "./github"
}
