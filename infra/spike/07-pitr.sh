#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Step 7+8 — WAL->GCS RPO under pod kill, then PITR to a NEW cluster (never in
# place). RPO = age of the newest archived WAL at the moment of the kill.
kubectl -n "$NS" annotate cluster spike-a cnpg.io/hibernation=off --overwrite
kubectl -n "$NS" wait cluster/spike-a --for=condition=Ready --timeout=600s

kubectl -n "$NS" exec spike-a-1 -c postgres -- \
  psql -d app -c "insert into pgbench_history select 2,2,2,2,now(),'pitr-marker' from generate_series(1,1000)" &
sleep 5
KILL_AT=$(date -u +%s)
kubectl -n "$NS" delete pod spike-a-1 --grace-period=0 --force
LAST_WAL=$(gsutil ls -l "gs://${WAL_BUCKET}/spike-a/wals/**" | sort -k2 | tail -2 | head -1 | awk '{print $2}')
LAST_WAL_S=$(python3 -c "from datetime import datetime,timezone; print(int((datetime.now(timezone.utc)-datetime.fromisoformat('${LAST_WAL}'.replace('Z','+00:00'))).total_seconds()))")
measure rpo_measured_s "$LAST_WAL_S" seconds

TARGET=$(date -u -v-10M +%FT%TZ 2>/dev/null || date -u -d '10 minutes ago' +%FT%TZ)
T0=$(now)
sed "s/__TARGET_TIME__/${TARGET}/" manifests/spike-pitr.yaml | kubectl apply -f -
kubectl -n "$NS" wait cluster/spike-pitr --for=condition=Ready --timeout=900s
T1=$(now)
measure pitr_to_new_s "$(elapsed "$T0" "$T1")" seconds
log "step 7/8 complete: kill at ${KILL_AT}; PITR target ${TARGET}"
