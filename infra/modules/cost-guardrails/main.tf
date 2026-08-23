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
    # EXPLICIT. The discovery doc marks calendarPeriod "Optional" and documents no
    # default, while the provider documents "Exactly one of calendar_period,
    # custom_period must be provided" — and the module set neither. MONTH is not a
    # new choice: it is what the live budget already uses and what a "monthly
    # budget" means in both other representations. An earlier version of the O2
    # writeup asserted MONTH was the API default; that was unsupported.
    calendar_period = "MONTH"
  }

  amount {
    specified_amount {
      # currency_code is DELIBERATELY UNSET. Google's discovery doc: "currency_code
      # is optional. If specified when creating a budget, it must match the
      # currency of the billing account... The currency_code is provided on
      # output." So the server supplies the account's own currency, correctly,
      # with no input required and no way to get it wrong. Setting it is the ONLY
      # path to an apply-time failure.
      #
      # Which means `units` is denominated in WHATEVER THE ACCOUNT USES — not
      # dollars. See the variable's docstring; the amount/currency pairing is a
      # founder decision recorded in O2, not something this module can infer.
      units = tostring(floor(var.monthly_budget_units))
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
