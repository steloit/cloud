terraform {
  required_version = ">= 1.9"
  required_providers {
    kubectl = {
      # alekc/kubectl, NOT gavinbunney/kubectl (ADR-0017) — and the LOCAL NAME
      # is not the point: a module whose source address differs from the root's
      # binds a SECOND, entirely unconfigured provider rather than inheriting the
      # root's. It then carries no host/token/CA, no `load_config_file = false`
      # and no `lazy_load`, so it falls back to whatever ~/.kube/config points at
      # — which is exactly the hazard the root provider block calls load-bearing,
      # for a CLUSTER-SCOPED object. Shipped that way once here; the only visible
      # trace was a gavinbunney entry appearing in both env lock files.
      source  = "alekc/kubectl"
      version = "~> 2.4"
    }
  }
}
