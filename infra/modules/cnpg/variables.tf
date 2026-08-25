variable "project_id" {
  type = string
}

variable "cell_id" {
  type = string
}

# The operator version rides the chart pin (chart 0.29.0 -> operator 1.30.0;
# the chart is the single source so CRDs + operator move atomically).

variable "operator_chart_version" {
  type        = string
  description = "cloudnative-pg chart version. 0.29.x ships operator 1.30.x — the HARD ceiling (architecture §3 v1.3 / ADR-0007 F6: in-tree Barman is REMOVED in 1.31). A bump past 0.29.x requires the barman-cloud plugin migration PR, never a drive-by."
  default     = "0.29.0"
  validation {
    # chart 0.30+ ships operator 1.31+ (in-tree Barman gone) — enforce the
    # ceiling in terraform, not just comments (review finding M1).
    condition     = can(regex("^0\\.29\\.", var.operator_chart_version)) || can(regex("^0\\.2[0-8]\\.", var.operator_chart_version))
    error_message = "operator_chart_version must stay <= 0.29.x (operator <= 1.30 — ADR-0007 F6 ceiling); the bump requires the barman-cloud plugin migration."
  }
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
  description = "Capacity, supplied by the env when control_plane = true (shape never carries numbers — no real default, review finding; null is 'not deploying a control plane here')"
  default     = null
}

