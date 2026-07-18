variable "project_id" {
  type = string
}

variable "zone" {
  type        = string
  description = "Zonal cluster location (ADR-0001 §14: zonal GKE Standard — free mgmt tier)"
}

variable "cell_id" {
  type = string
}

variable "network_id" {
  type = string
}

variable "subnet_id" {
  type = string
}

variable "deletion_protection" {
  type        = bool
  description = "true for cell0; false for the destroyable dev env"
}

# --- capacity (always supplied by the env; shape never carries numbers) ---

variable "core_machine_type" {
  type = string
}

variable "core_min_nodes" {
  type        = number
  description = "A1.6: floor 1 — the core pool never scales to zero"
}

variable "core_max_nodes" {
  type = number
}

variable "storage_machine_type" {
  type = string
}

variable "storage_node_count" {
  type = number
}

variable "storage_local_ssd_count" {
  type = number
}

variable "workload_machine_type" {
  type = string
}

variable "workload_max_nodes" {
  type = number
}
