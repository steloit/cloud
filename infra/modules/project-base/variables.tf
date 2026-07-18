variable "project_id" {
  type        = string
  description = "GCP project id (supplied when P1 lands; never hardcoded)"
}

variable "region" {
  type        = string
  description = "Home region for buckets/keyring"
}

variable "cell_id" {
  type        = string
  description = "Cell identity label carried by every resource (US-1.1 / INF-001 invariant 1)"
}

variable "github_repo" {
  type        = string
  description = "owner/repo allowed to federate for CI plan (workload-identity, zero static keys)"
  default     = "steloit/cloud"
}
