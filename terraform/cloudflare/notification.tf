resource "cloudflare_notification_policy" "ddos_attack" {
  account_id  = var.account_id
  name        = "DDoS Attack Alert"
  alert_type  = "dos_attack_l7"
  enabled     = true
  description = "Alert on HTTP DDoS attacks"

  mechanisms = {
    email = [{
      id = var.notification_email
    }]
  }
}

resource "cloudflare_notification_policy" "ssl_alert" {
  account_id  = var.account_id
  name        = "SSL Certificate Alert"
  alert_type  = "universal_ssl_event_type"
  enabled     = true
  description = "Alert on SSL certificate events"

  mechanisms = {
    email = [{
      id = var.notification_email
    }]
  }
}

resource "cloudflare_notification_policy" "origin_monitoring" {
  account_id  = var.account_id
  name        = "Origin Error Monitoring"
  alert_type  = "real_origin_monitoring"
  enabled     = true
  description = "Alert on origin server errors"

  mechanisms = {
    email = [{
      id = var.notification_email
    }]
  }
}

resource "cloudflare_notification_policy" "web_analytics" {
  account_id  = var.account_id
  name        = "Web Analytics Weekly Summary"
  alert_type  = "web_analytics_metrics_update"
  enabled     = true
  description = "Weekly web analytics summary"

  mechanisms = {
    email = [{
      id = var.notification_email
    }]
  }
}

resource "cloudflare_notification_policy" "maintenance" {
  account_id  = var.account_id
  name        = "Cloudflare Maintenance"
  alert_type  = "maintenance_event_notification"
  enabled     = true
  description = "Cloudflare maintenance notifications"

  mechanisms = {
    email = [{
      id = var.notification_email
    }]
  }
}

resource "cloudflare_notification_policy" "incident" {
  account_id  = var.account_id
  name        = "Cloudflare Incident"
  alert_type  = "incident_alert"
  enabled     = true
  description = "Cloudflare incident notifications"

  mechanisms = {
    email = [{
      id = var.notification_email
    }]
  }
}

resource "cloudflare_notification_policy" "security_insights" {
  account_id  = var.account_id
  name        = "Security Insights"
  alert_type  = "security_insights_alert"
  enabled     = true
  description = "Security insights notifications"

  mechanisms = {
    email = [{
      id = var.notification_email
    }]
  }
}
