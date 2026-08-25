variable "project_id" {
  type = string
}

variable "cell_id" {
  type = string
}

variable "billing_account" {
  type        = string
  description = "Billing account id (P1 gate — the module is inert while empty)"
  default     = ""
}

variable "monthly_budget_units" {
  type        = number
  description = <<-EOT
    Budget amount the 50/80/100% alerts fire against, in the BILLING ACCOUNT'S OWN
    CURRENCY — not dollars.

    Renamed from monthly_budget_usd because that name was load-bearing and wrong:
    currency_code is unset (the server supplies the account currency), so a value
    chosen as dollars becomes that many rupees, yen or pesos. 300 on an INR account
    is about US$3.60, and the alerts would fire permanently.

    The amount and the account currency are a PAIR, and choosing them is a founder
    decision — see O2. See docs/founder-config.md for the account identifiers.
  EOT
}


variable "alert_emails" {
  type        = list(string)
  description = "Founder emails for budget notifications"
  default     = []
}
