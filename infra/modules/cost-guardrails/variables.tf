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

variable "alert_emails" {
  type        = list(string)
  description = "Founder emails for budget notifications"
  default     = []
}
