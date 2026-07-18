variable "project_id" {
  type = string
}

variable "cell_id" {
  type = string
}

variable "operator_version" {
  type        = string
  description = "CNPG operator — pinned <= 1.30 (architecture §3: in-tree Barman until the barman-cloud plugin issues close; version bumps are a reviewed knob)"
  default     = "1.30.0"
}
