#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Step 9 — nothing survives the week (O2). Clusters, snapshots, operator, ns.
kubectl delete -f manifests/spike-b-recovery.yaml --ignore-not-found
kubectl -n "$NS" delete cluster spike-pitr --ignore-not-found
kubectl delete -f manifests/spike-a.yaml --ignore-not-found
kubectl -n "$NS" delete volumesnapshot spike-a-snap --ignore-not-found
kubectl delete namespace "$NS" --ignore-not-found
log "teardown complete — verify in console: no spike PVs/PVCs remain"
