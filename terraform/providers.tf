provider "google" {
  project = var.project_id
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

provider "github" {
  owner = "hippocampus-dev"
  token = var.github_token
}
