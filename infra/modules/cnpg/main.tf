# T1.2 — the database platform layer (ADR-0003 · ADR-0007/A6 ratified
# 2026-07-19): CNPG operator + the proven PD-CSI storage/snapshot classes +
# (optionally) the control-plane CNPG cluster. One apply path: everything is
# Terraform (helm/kubernetes providers configured by the env from gke-cell
# outputs). ZFS-LocalPV install is NOT here — it is the Cell-1 density option
# (INF-001 A6), kit retained under infra/spike/.

locals {
  labels = {
    cell_id    = var.cell_id
    managed_by = "steloit-terraform"
  }
  # ONE identity source for the control-plane DB: templated into the Cluster +
  # ScheduledBackup yaml AND the WI member string (review finding: cross-file
  # identity coupling fails only at runtime).
  cp_namespace = "control-plane"
  cp_cluster   = "control-plane"
}

# --- CNPG operator -----------------------------------------------------------
# Pinned 1.30.x (architecture §3 v1.3): in-tree Barman is REMOVED in 1.31
# (ADR-0007 F6) — this pin is a HARD CEILING, and the barman-cloud plugin
# migration is the tracked exit (a reviewed PR, never a drive-by bump).
resource "helm_release" "cnpg_operator" {
  name             = "cnpg"
  repository       = "https://cloudnative-pg.github.io/charts"
  chart            = "cloudnative-pg"
  version          = var.operator_chart_version # 0.29.0 ships operator 1.30.0 (charts index appVersion — verified 2026-07-19); the chart pins operator + CRDs ATOMICALLY, so no image override (review finding: an image.tag override risks CRD/operator skew)
  namespace        = "cnpg-system"
  create_namespace = true
  # NOTE: `set {}` block syntax is helm provider 2.x — a 3.x bump changes this.
}

# --- storage: the T1.0-proven classes, VERBATIM ------------------------------
# Source of truth is infra/k8s/storage/*.yaml (the yaml the spike proved);
# Terraform applies the same files — one truth, one apply path.
resource "kubernetes_manifest" "pd_storageclass" {
  manifest = yamldecode(file("${path.module}/../../k8s/storage/pd-storageclass.yaml"))
}

resource "kubernetes_manifest" "pd_snapshotclass" {
  manifest = yamldecode(file("${path.module}/../../k8s/storage/pd-snapshotclass.yaml"))
}

# --- control-plane database (invariant 10: its OWN bucket) -------------------
# Only where the control plane lives (dev today): a single-instance CNPG
# cluster, PITR to the SEPARATE wal-control bucket, and — load-bearing from
# day one (ADR-0007 F3: WAL alone is NOT restorable) — a ScheduledBackup.

resource "google_service_account" "cnpg_control" {
  count        = var.control_plane ? 1 : 0
  project      = var.project_id
  account_id   = "cnpg-control"
  display_name = "CNPG control-plane WAL/backup archiver (workload identity; zero static keys — D5)"
}

resource "google_storage_bucket_iam_member" "cnpg_control_wal" {
  count  = var.control_plane ? 1 : 0
  bucket = var.wal_control_bucket
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.cnpg_control[0].email}"
}

# CNPG runs the cluster under KSA <cluster-name> in its namespace (F5:
# workload identity is a hard prerequisite for barman→GCS). The member string
# derives from the SAME locals templated into the yaml — one identity source.
resource "google_service_account_iam_member" "cnpg_control_wif" {
  count              = var.control_plane ? 1 : 0
  service_account_id = google_service_account.cnpg_control[0].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[${local.cp_namespace}/${local.cp_cluster}]"
}

resource "kubernetes_namespace" "control_plane" {
  count = var.control_plane ? 1 : 0
  metadata {
    name   = local.cp_namespace
    labels = local.labels
  }
}

resource "kubernetes_manifest" "control_plane_cluster" {
  count = var.control_plane ? 1 : 0
  lifecycle {
    precondition {
      condition     = var.control_plane_storage_size != null
      error_message = "control_plane = true requires control_plane_storage_size from the env (capacity lives in envs)."
    }
  }
  manifest = yamldecode(templatefile("${path.module}/../../k8s/control-plane/cnpg-cluster.yaml", {
    namespace          = local.cp_namespace
    cluster_name       = local.cp_cluster
    wal_control_bucket = var.wal_control_bucket
    gsa_email          = google_service_account.cnpg_control[0].email
    storage_size       = var.control_plane_storage_size
  }))
  depends_on = [
    helm_release.cnpg_operator,
    kubernetes_manifest.pd_storageclass,
    kubernetes_namespace.control_plane,
  ]
}

resource "kubernetes_manifest" "control_plane_backup_schedule" {
  count = var.control_plane ? 1 : 0
  manifest = yamldecode(templatefile("${path.module}/../../k8s/control-plane/cnpg-scheduled-backup.yaml", {
    namespace    = local.cp_namespace
    cluster_name = local.cp_cluster
  }))
  depends_on = [kubernetes_manifest.control_plane_cluster]
}
