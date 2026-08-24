# T1.2 — the database platform layer (ADR-0003 · ADR-0007/A6 ratified
# 2026-07-19): CNPG operator + the proven PD-CSI storage/snapshot classes +
# (optionally) the control-plane CNPG cluster. One apply TOOL (Terraform) — the apply itself is STAGED
# (infra/README.md): everything is Terraform (helm/kubernetes providers configured by the env from gke-cell
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
# Terraform applies the same files — one truth, one tool, ONE PASS (T1.4a).
#
# WHY kubectl_manifest AND NOT kubernetes_manifest — this is the whole of T1.4a.
# `kubernetes_manifest` resolves its GroupVersionKind against the live API at
# PLAN time, to build Terraform's type information from the OpenAPI schema. On a
# from-zero apply there is no API yet (the cluster is created by this same
# apply), and for a CR there is no CRD yet either — so the plan fails before
# anything is created:
#
#   Error: API did not recognize GroupVersionKind from manifest
#          (CRD may not be installed)
#
# That is an acknowledged, still-open limitation (hashicorp/terraform-provider-
# kubernetes#1367, #2597), not a transient error, and `depends_on` cannot fix it:
# validation happens before dependency resolution. `kubectl_manifest` applies
# raw YAML the way `kubectl apply` does and resolves nothing at plan time, so the
# cluster, the CRDs and the objects that need them all land in one pass — which
# is what HashiCorp's own GKE example does for everything except this resource.
#
# It applies to the STORAGE classes too, not only the CNPG CRs. A `kubernetes_
# manifest` needs the API at plan time whatever its kind, so on a genuine
# from-zero apply these two failed as well; the original report only named the
# CNPG resources because that apply already ran `-target=module.gke_cell` first.
resource "kubectl_manifest" "pd_storageclass" {
  yaml_body = file("${path.module}/../../k8s/storage/pd-storageclass.yaml")
}

resource "kubectl_manifest" "pd_snapshotclass" {
  yaml_body = file("${path.module}/../../k8s/storage/pd-snapshotclass.yaml")
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

resource "kubectl_manifest" "control_plane_cluster" {
  count = var.control_plane ? 1 : 0
  lifecycle {
    precondition {
      condition     = var.control_plane_storage_size != null
      error_message = "control_plane = true requires control_plane_storage_size from the env (capacity lives in envs)."
    }
  }
  yaml_body = templatefile("${path.module}/../../k8s/control-plane/cnpg-cluster.yaml", {
    namespace          = local.cp_namespace
    cluster_name       = local.cp_cluster
    wal_control_bucket = var.wal_control_bucket
    gsa_email          = google_service_account.cnpg_control[0].email
    storage_size       = var.control_plane_storage_size
  })
  # The CRDs arrive with the operator's chart, in this same apply. depends_on is
  # what orders that; kubectl_manifest is what makes the ordering SUFFICIENT.
  depends_on = [
    helm_release.cnpg_operator,
    kubectl_manifest.pd_storageclass,
    kubernetes_namespace.control_plane,
  ]
}

resource "kubectl_manifest" "control_plane_backup_schedule" {
  count = var.control_plane ? 1 : 0
  yaml_body = templatefile("${path.module}/../../k8s/control-plane/cnpg-scheduled-backup.yaml", {
    namespace    = local.cp_namespace
    cluster_name = local.cp_cluster
  })
  depends_on = [kubectl_manifest.control_plane_cluster]
}
