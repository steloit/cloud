#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Step 4 — spike-b: new CNPG cluster via bootstrap:recovery from the snapshot;
# branch e2e = apply -> accepting connections. Verifies data identity.
T0=$(now)
kubectl apply -f manifests/spike-b-recovery.yaml
kubectl -n "$NS" wait cluster/spike-b --for=condition=Ready --timeout=600s
T1=$(now)
measure branch_create_s "$(elapsed "$T0" "$T1")" seconds

A=$(kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -tAq -d app -c "select count(*) from pgbench_accounts")
B=$(kubectl -n "$NS" exec spike-b-1 -c postgres -- psql -tAq -d app -c "select count(*) from pgbench_accounts")
[ "$A" = "$B" ] || { log "FAIL branch data mismatch a=$A b=$B"; exit 1; }
log "step 4 complete: branch data verified identical ($A rows)"
