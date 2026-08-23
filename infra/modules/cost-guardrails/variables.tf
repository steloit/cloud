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

variable "monthly_budget_usd" {
  type        = number
  description = "Budget amount the 50/80/100% alerts fire against (trial-credit sized; capacity, set by the env)"
}

variable "budget_currency" {
  type        = string
  description = <<-EOT
    Currency for the budget amount. MUST equal the billing account's own currency —
    the Cloud Billing Budget API rejects any other value; this is not a choice.

    Left as a variable rather than hardcoded because the hardcoded "USD" contradicted
    the only observable evidence: the sole budget on billing account
    016006-61AFB9-0DD7E7 is denominated in INR. The account's currency could not be
    read directly (the Cloud Billing API is not enabled on steloit-dev), so this is
    NOT asserted as USD-is-wrong — it is asserted that a hardcoded value which
    disagrees with the only live sample will fail at apply time, and the value
    belongs where whoever supplies `billing_account` (P1) can set it in the same
    breath. See O2.
  EOT
}

variable "alert_emails" {
  type        = list(string)
  description = "Founder emails for budget notifications"
  default     = []
}
