#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Step 5 — CoW divergence: ~10% new writes on each side, then the ZFS delta.
for c in spike-a spike-b; do
  kubectl -n "$NS" exec "${c}-1" -c postgres -- \
    psql -d app -c "insert into pgbench_history select 1,1,1,1,now(),repeat('x',80) from generate_series(1,500000)"
done
# Per-volume ZFS accounting from the zfs-localpv node agent (zvol used bytes).
NODE_POD=$(kubectl -n openebs get pod -l name=openebs-zfs-node -o jsonpath='{.items[0].metadata.name}')
kubectl -n openebs exec "$NODE_POD" -c openebs-zfs-plugin -- zfs list -Hp -o name,used,refer | tee logs/zfs-list.txt
DELTA=$(awk '/spike-b/ {print $2; exit}' logs/zfs-list.txt)
measure cow_delta_bytes "${DELTA:-0}" bytes
log "step 5 complete: divergence written; raw zfs accounting in logs/zfs-list.txt (arithmetic shown in the ADR)"
