#!/usr/bin/env bash
# T1.0 substrate spike — PD-CSI branch (the durable alternative to ZFS-LocalPV;
# see ADR-0007). Measures snapshot time, branch(recovery) time, data identity,
# restart latency, and the snapshot's incremental byte size (for the CoW cost
# analysis). NOT set -e on the body — every run guarantees teardown.
set -uo pipefail
cd "$(dirname "$0")"
PROJECT=steloit-dev ZONE=us-central1-a CLUSTER="${SPIKE_CLUSTER:-spike-e2}" NS=spike
CNPG=1.30.0
RESULTS=results/pdcsi.log
mkdir -p results; : > "$RESULTS"
now(){ python3 -c 'import time;print(f"{time.monotonic():.3f}")'; }
el(){ python3 -c "print(f'{$2-$1:.1f}')"; }
measure(){ echo "MEASURE $1=$2 unit=$3" | tee -a "$RESULTS"; }
log(){ echo "$(date -u +%FT%TZ) $*" | tee -a "$RESULTS"; }

teardown(){
  log "=== TEARDOWN (in-cluster) ==="
  kubectl delete volumesnapshot -n "$NS" --all --wait=false 2>/dev/null || true
  kubectl delete ns "$NS" --wait=false 2>/dev/null || true
  log "namespace + snapshot deletion requested (cluster teardown is the separate final step)"
}
trap teardown EXIT

gcloud container clusters get-credentials "$CLUSTER" --zone "$ZONE" --project "$PROJECT" >/dev/null 2>&1
log "=== step 1: CNPG operator + PD-CSI storage/snapshot classes ==="
kubectl apply --server-side -f "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.30/releases/cnpg-${CNPG}.yaml" >/dev/null
kubectl -n cnpg-system rollout status deployment/cnpg-controller-manager --timeout=300s || { log "CNPG operator failed"; exit 1; }
kubectl create ns "$NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl apply -f manifests/pd-storageclass.yaml -f manifests/pd-snapshotclass.yaml >/dev/null
log "operator ready"

log "=== step 2: spike-a (Dev-shape CNPG on PD) + load ~300MB ==="
kubectl apply -f manifests/spike-a-pd.yaml >/dev/null
kubectl -n "$NS" wait cluster/spike-a --for=condition=Ready --timeout=600s || { log "spike-a not ready"; exit 1; }
kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -q -d app -c \
  "CREATE TABLE load AS SELECT g id, md5(g::text) h, repeat('x',200) pad FROM generate_series(1,1200000) g;" 2>&1 | tail -1
BASE=$(kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -tAq -d app -c "select count(*) from load")
DBSZ=$(kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -tAq -d app -c "select pg_database_size('app')")
measure baseline_rows "$BASE" rows
measure baseline_db_bytes "$DBSZ" bytes

log "=== step 3: VolumeSnapshot (PD-CSI, timed) ==="
PVC=$(kubectl -n "$NS" get pvc -l cnpg.io/cluster=spike-a -o jsonpath='{.items[0].metadata.name}')
T0=$(now)
cat <<YAML | kubectl apply -f - >/dev/null
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata: {name: spike-a-snap, namespace: ${NS}}
spec: {volumeSnapshotClassName: pd-snapclass, source: {persistentVolumeClaimName: ${PVC}}}
YAML
kubectl -n "$NS" wait volumesnapshot/spike-a-snap --for=jsonpath='{.status.readyToUse}'=true --timeout=600s || log "WARN snapshot not ready in time"
T1=$(now)
measure snapshot_create_s "$(el "$T0" "$T1")" seconds
RESTORE_BYTES=$(kubectl -n "$NS" get volumesnapshot spike-a-snap -o jsonpath='{.status.restoreSize}' 2>/dev/null || echo "?")
measure snapshot_restore_size "$RESTORE_BYTES" bytes

log "=== step 4: branch spike-b via bootstrap:recovery (timed) ==="
T0=$(now)
kubectl apply -f manifests/spike-b-pd.yaml >/dev/null
kubectl -n "$NS" wait cluster/spike-b --for=condition=Ready --timeout=600s || { log "spike-b not ready"; }
T1=$(now)
measure branch_create_s "$(el "$T0" "$T1")" seconds
B=$(kubectl -n "$NS" exec spike-b-1 -c postgres -- psql -tAq -d app -c "select count(*) from load" 2>/dev/null || echo x)
if [ "$B" = "$BASE" ]; then measure branch_data_identical 1 bool; log "branch verified: $B rows == baseline"; else measure branch_data_identical 0 bool; log "BRANCH MISMATCH b=$B base=$BASE"; fi

log "=== step 5: restart/wake latency (delete primary pod -> accepting) ==="
T0=$(now)
kubectl -n "$NS" delete pod spike-b-1 --wait=false >/dev/null 2>&1
sleep 2
for i in $(seq 1 120); do
  kubectl -n "$NS" exec spike-b-1 -c postgres -- pg_isready -q 2>/dev/null && break
  sleep 2
done
T1=$(now)
measure restart_to_ready_s "$(el "$T0" "$T1")" seconds

log "=== step 6: PD sizes (cost basis: base disk vs branch disk vs snapshot) ==="
gcloud compute snapshots list --project="$PROJECT" --format="value(name,diskSizeGb,storageBytes)" 2>/dev/null | tee -a "$RESULTS" || true
gcloud compute disks list --project="$PROJECT" --filter="zone:( $ZONE )" --format="value(name,sizeGb,type)" 2>/dev/null | tee -a "$RESULTS" || true

log "=== RESULTS ==="; grep MEASURE "$RESULTS"
log "spike measurement complete"
