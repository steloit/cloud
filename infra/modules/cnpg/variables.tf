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

variable "operator_chart_version" {
  type        = string
  description = "cloudnative-pg helm chart version shipping operator 1.30.x (the <=1.30 ceiling — ADR-0007 F6; in-tree Barman removed in 1.31)"
  default     = "0.26.0"
}

variable "control_plane" {
  type        = bool
  description = "Deploy the control-plane CNPG cluster here (dev today; invariant 10 gives it its OWN wal-control bucket)"
  default     = false
}

variable "wal_control_bucket" {
  type        = string
  description = "The control-plane DB's own PITR bucket (INF-001 invariant 10)"
  default     = ""
}

variable "control_plane_storage_size" {
  type        = string
  description = "Capacity, supplied by the env (shape never carries numbers)"
  default     = "10Gi"
}
