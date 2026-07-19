#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Step 3 — ZFS CoW VolumeSnapshot of spike-a's PVC; timed.
PVC=$(kubectl -n "$NS" get pvc -l cnpg.io/cluster=spike-a -o jsonpath='{.items[0].metadata.name}')
T0=$(now)
cat <<YAML | kubectl apply -f -
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata: {name: spike-a-snap, namespace: ${NS}}
spec:
  volumeSnapshotClassName: zfs-snapclass
  source: {persistentVolumeClaimName: ${PVC}}
YAML
kubectl -n "$NS" wait volumesnapshot/spike-a-snap --for=jsonpath='{.status.readyToUse}'=true --timeout=300s
T1=$(now)
measure snapshot_create_s "$(elapsed "$T0" "$T1")" seconds
