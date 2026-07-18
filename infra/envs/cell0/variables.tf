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
