variable "project_id" {
  type        = string
  description = "Supplied when P1 lands (never hardcoded — T1.1 common-mistakes rule)"
}

variable "region" {
  type    = string
  default = "asia-south1"
}

variable "zone" {
  type    = string
  default = "asia-south1-a"
}

variable "cell_id" {
  type    = string
  default = "cell0"
}

variable "content_domain" {
  type        = string
  description = "Empty until P2 registers the customer-content eTLD+1 (A2.4)"
  default     = ""
}

variable "billing_account" {
  type        = string
  description = "P1 gate — cost guardrails are inert while empty"
  default     = ""
}

variable "alert_emails" {
  type    = list(string)
  default = []
}


variable "monthly_budget_units" {
  type        = number
  description = <<-EOT
    Budget amount, in the BILLING ACCOUNT'S OWN CURRENCY (currency_code is unset;
    the server supplies it). NO DEFAULT, deliberately — same idiom as project_id:
    supplied when P1 lands, never hardcoded.

    This is the guard, not the docstring. A literal here meant an operator could
    supply billing_account alone and silently get a 300-unit budget — which on an
    INR account is about US$3.60, alerting permanently. With no default,
    `terraform plan` refuses until the amount is chosen in the same breath as the
    account, which is when its currency becomes knowable. See O2 decision #5.
  EOT
}
