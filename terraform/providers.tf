provider "google" {
  project = var.project_id
  # The Billing Budgets API rejects user credentials without a quota project: https://github.com/hashicorp/terraform-provider-google/blob/v6.50.0/website/docs/r/billing_budget.html.markdown
  billing_project       = var.project_id
  user_project_override = true
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

provider "github" {
  owner = "hippocampus-dev"
  token = var.github_token
}
