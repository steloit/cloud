variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "cell_id" {
  type = string
}

variable "subnet_cidr" {
  type        = string
  description = "Cell subnet CIDR (capacity decision — set by the env)"
}

variable "content_domain" {
  type        = string
  description = "Customer-content eTLD+1 (A2.4). Empty until P2 registers it — the zone is gated on this."
  default     = ""
}

