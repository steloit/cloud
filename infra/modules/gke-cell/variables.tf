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

  # A VALIDATION, not a comment, and not a test assertion — because the value is
  # ENV-supplied and a test can only pin the envs that exist. Review measured
  # `core_min_nodes = 0` in either env as fully green: the module test asserts
  # min_node_count >= 1 against the tftest file's OWN input of 1, which is
  # tautological with respect to every deployed value. A variable validation is
  # the only place that holds for the next env too, before it is written.
  #
  # A1.6 is a floor because the core pool runs the api, the operators and
  # observability: scaling it to zero takes the control plane of the cell down
  # with it, and nothing schedules it back.
  validation {
    condition     = var.core_min_nodes >= 1
    error_message = "core_min_nodes must be at least 1 (A1.6): the core pool runs the api, the operators and observability, and a cell whose core pool scales to zero cannot bring itself back."
  }
}

variable "core_max_nodes" {
  type = number

  validation {
    condition     = var.core_max_nodes >= var.core_min_nodes
    error_message = "core_max_nodes must be >= core_min_nodes; GKE rejects min > max at apply, and a mocked plan has no backstop for it."
  }
}

variable "storage_machine_type" {
  type = string
}

variable "storage_driver" {
  type        = string
  description = "pd (canonical dev/alpha — ADR-0007/A6) | zfs (Cell-1 density option: local SSD + OpenEBS ZFS-LocalPV, measured trigger required)"
  default     = "pd"
  validation {
    condition     = contains(["pd", "zfs"], var.storage_driver)
    error_message = "storage_driver must be pd or zfs."
  }
}

variable "storage_node_count" {
  type = number

  validation {
    condition     = var.storage_node_count >= 1
    error_message = "storage_node_count must be at least 1: the db-storage pool is where CNPG runs, and zero nodes is a silently broken cell rather than an apply error."
  }
}

variable "storage_local_ssd_count" {
  type        = number
  description = "Only consumed when storage_driver = zfs (Cell-1 knob)"
  default     = 0
}

variable "workload_machine_type" {
  type = string
}

variable "workload_max_nodes" {
  type = number

  validation {
    condition     = var.workload_max_nodes >= 1
    error_message = "workload_max_nodes must be at least 1: an autoscaled workload pool capped at zero accepts no customer workloads."
  }
}
