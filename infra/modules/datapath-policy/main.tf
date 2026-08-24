# THE DATAPATH'S OBSERVABILITY — the other half of choosing Dataplane V2.
#
# ADR-0015's FIRST stated reason for Dataplane V2 over Calico is denied-connection
# logging. Until US-3.3c that rationale rested on a capability nothing installed:
# infra/k8s/policy/network-logging.yaml was authored, reviewed, and applied by
# NOTHING (`grep -rn network-logging` over infra/ and services/ returned only the
# file itself). A decision's primary justification cannot be a file on disk.
#
# It lives in its own module rather than in gke-cell because gke-cell is pure
# GCP — it has no Kubernetes provider and cannot acquire one without the
# chicken-and-egg of configuring a provider from the cluster it creates. It is
# not in `cnpg` either: network logging is a property of the datapath, not of
# Postgres, and putting it there would make a database module own the cell's
# audit trail.
#
# Applied with kubectl_manifest, not kubernetes_manifest, for the reason T1.4a
# recorded (ADR-0017): NetworkLogging is a CRD GKE installs on the cluster this
# same apply creates, so a plan-time GroupVersionKind lookup has nothing to
# resolve against.
resource "kubectl_manifest" "network_logging" {
  yaml_body = file("${path.module}/../../k8s/policy/network-logging.yaml")
}
