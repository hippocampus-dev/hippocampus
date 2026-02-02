variable "account_id" {
  description = "Cloudflare Account ID"
  type        = string
}

variable "pages_domain" {
  description = "Custom domain for Cloudflare Pages"
  type        = string
  default     = "www.kaidotio.dev"
}

variable "notification_email" {
  description = "Email address for notifications"
  type        = string
  default     = "notification@kaidotio.dev"
}
