# O2: billing alerts at 50/80/100% of the trial credit — the founders hear
# about spend BEFORE it becomes a surprise (the platform's own bill-shock
# rule applied to itself). Gated on billing_account (P1).
locals {
  enabled = var.billing_account != ""
}

resource "google_monitoring_notification_channel" "founders" {
  for_each     = local.enabled ? toset(var.alert_emails) : []
  project      = var.project_id
  display_name = "founder ${each.value}"
  type         = "email"
  labels = {
    email_address = each.value
  }
}

resource "google_billing_budget" "cell" {
  count           = local.enabled ? 1 : 0
  billing_account = var.billing_account
  display_name    = "${var.cell_id} budget"

  budget_filter {
    projects = ["projects/${var.project_id}"]
  }

  amount {
    specified_amount {
      # Must match the billing account's currency; the API rejects anything else.
      currency_code = var.budget_currency
      units         = tostring(floor(var.monthly_budget_usd))
    }
  }

  threshold_rules {
    threshold_percent = 0.5
  }
  threshold_rules {
    threshold_percent = 0.8
  }
  threshold_rules {
    threshold_percent = 1.0
  }

  all_updates_rule {
    monitoring_notification_channels = [for c in google_monitoring_notification_channel.founders : c.id]
    disable_default_iam_recipients   = false
  }
}
