#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Step 2 — spike-a: Dev-shaped CNPG cluster + pgbench dataset (~750 MB).
kubectl apply -f manifests/spike-a.yaml
kubectl -n "$NS" wait cluster/spike-a --for=condition=Ready --timeout=600s

kubectl -n "$NS" exec spike-a-1 -c postgres -- pgbench -i -s 50 app
BASELINE=$(kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -tAq -d app -c "select pg_database_size('app')")
measure baseline_bytes "$BASELINE" bytes
log "step 2 complete: spike-a healthy, pgbench s=50 loaded"
