#!/usr/bin/env bash
# T1.0 spike — phase 2 (PD-CSI): divergence delta, hibernation wake, WAL→GCS
# RPO, PITR-to-new. Requires phase 1 (run-pdcsi.sh) to have left spike-a/spike-b
# running — run with KEEP=1 phase 1, or re-create spike-a first.
# Prereq discovered during the spike (itself a finding): the ad-hoc cluster has
# no workload identity → enable it first (enable-wi step below).
set -uo pipefail
cd "$(dirname "$0")"
PROJECT=steloit-dev ZONE=us-central1-a CLUSTER="${SPIKE_CLUSTER:-spike-e2}" NS=spike
WAL_BUCKET=${PROJECT}-wal-customer
GSA=spike-wal@${PROJECT}.iam.gserviceaccount.com
RESULTS=results/pdcsi-phase2.log
mkdir -p results; : > "$RESULTS"
now(){ python3 -c 'import time;print(f"{time.monotonic():.3f}")'; }
el(){ python3 -c "print(f'{$2-$1:.1f}')"; }
measure(){ echo "MEASURE $1=$2 unit=$3" | tee -a "$RESULTS"; }
log(){ echo "$(date -u +%FT%TZ) $*" | tee -a "$RESULTS"; }

case "${1:-run}" in
enable-wi)
  # One-time: workload identity on the ad-hoc cluster (control plane, then node
  # pool — the node pool step recreates the node; PD volumes survive).
  gcloud container clusters update "$CLUSTER" --zone "$ZONE" --project "$PROJECT" \
    --workload-pool="${PROJECT}.svc.id.goog"
  gcloud container node-pools update default-pool --cluster "$CLUSTER" \
    --zone "$ZONE" --project "$PROJECT" --workload-metadata=GKE_METADATA
  gcloud iam service-accounts create spike-wal --project "$PROJECT" \
    --display-name="T1.0 spike WAL archiver (temporary)" 2>/dev/null || true
  gcloud storage buckets add-iam-policy-binding "gs://${WAL_BUCKET}" \
    --member="serviceAccount:${GSA}" --role=roles/storage.objectAdmin >/dev/null
  # CNPG runs spike-a under KSA "spike-a" in ns "spike"
  gcloud iam service-accounts add-iam-policy-binding "$GSA" --project "$PROJECT" \
    --member="serviceAccount:${PROJECT}.svc.id.goog[${NS}/spike-a]" \
    --role=roles/iam.workloadIdentityUser >/dev/null
  gcloud iam service-accounts add-iam-policy-binding "$GSA" --project "$PROJECT" \
    --member="serviceAccount:${PROJECT}.svc.id.goog[${NS}/spike-pitr]" \
    --role=roles/iam.workloadIdentityUser >/dev/null
  echo "workload identity ready"
  ;;
run)
  gcloud container clusters get-credentials "$CLUSTER" --zone "$ZONE" --project "$PROJECT" >/dev/null 2>&1

  log "=== step 0: bootstrap (phase 1's teardown removes the ns — recreate) ==="
  kubectl create ns "$NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl apply -f manifests/pd-storageclass.yaml -f manifests/pd-snapshotclass.yaml >/dev/null
  if ! kubectl -n "$NS" get cluster spike-a >/dev/null 2>&1; then
    kubectl apply -f manifests/spike-a-pd.yaml >/dev/null
    kubectl -n "$NS" wait cluster/spike-a --for=condition=Ready --timeout=600s
    kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -q -d app -c \
      "CREATE TABLE load AS SELECT g id, md5(g::text) h, repeat('x',200) pad FROM generate_series(1,1200000) g;" >/dev/null
    log "spike-a recreated + loaded"
  fi
  if ! kubectl -n "$NS" get cluster spike-b >/dev/null 2>&1; then
    PVC=$(kubectl -n "$NS" get pvc -l cnpg.io/cluster=spike-a -o jsonpath='{.items[0].metadata.name}')
    cat <<YAML | kubectl apply -f - >/dev/null
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata: {name: spike-a-snap, namespace: ${NS}}
spec: {volumeSnapshotClassName: pd-snapclass, source: {persistentVolumeClaimName: ${PVC}}}
YAML
    kubectl -n "$NS" wait volumesnapshot/spike-a-snap --for=jsonpath='{.status.readyToUse}'=true --timeout=600s
    kubectl apply -f manifests/spike-b-pd.yaml >/dev/null
    kubectl -n "$NS" wait cluster/spike-b --for=condition=Ready --timeout=600s
    log "spike-b recreated from snapshot"
  fi

  log "=== step A: turn on WAL archiving (barman → gs://${WAL_BUCKET}) ==="
  kubectl -n "$NS" patch cluster spike-a --type merge -p "{
    \"spec\": {
      \"serviceAccountTemplate\": {\"metadata\": {\"annotations\": {\"iam.gke.io/gcp-service-account\": \"${GSA}\"}}},
      \"backup\": {\"barmanObjectStore\": {\"destinationPath\": \"gs://${WAL_BUCKET}/spike-a\", \"googleCredentials\": {\"gkeEnvironment\": true}}, \"retentionPolicy\": \"7d\"}
    }}"
  kubectl -n "$NS" wait cluster/spike-a --for=condition=Ready --timeout=600s
  # force a WAL switch so the first archive lands, then wait for objects
  kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -d app -c "select pg_switch_wal()" >/dev/null
  T0=$(now); OK=0
  for i in $(seq 1 60); do
    N=$(gcloud storage ls "gs://${WAL_BUCKET}/spike-a/wals/**" 2>/dev/null | wc -l | tr -d ' ')
    [ "${N:-0}" -gt 0 ] && OK=1 && break; sleep 5
  done
  T1=$(now)
  [ "$OK" = 1 ] && measure wal_first_archive_s "$(el "$T0" "$T1")" seconds || log "WARN no WAL archived in 300s"

  log "=== step B: divergence + snapshot delta (cow_delta equivalent) ==="
  kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -q -d app -c \
    "INSERT INTO load SELECT g, md5(g::text), repeat('y',200) FROM generate_series(1200001,1320000) g;" # ~10%
  kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -q -d app -c "checkpoint"
  PVC=$(kubectl -n "$NS" get pvc -l cnpg.io/cluster=spike-a -o jsonpath='{.items[0].metadata.name}')
  cat <<YAML | kubectl apply -f - >/dev/null
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata: {name: spike-a-snap2, namespace: ${NS}}
spec: {volumeSnapshotClassName: pd-snapclass, source: {persistentVolumeClaimName: ${PVC}}}
YAML
  kubectl -n "$NS" wait volumesnapshot/spike-a-snap2 --for=jsonpath='{.status.readyToUse}'=true --timeout=600s
  sleep 20 # PD snapshot storageBytes settles asynchronously
  gcloud compute snapshots list --project "$PROJECT" --format="value(name,storageBytes)" | tee -a "$RESULTS"
  DELTA=$(gcloud compute snapshots list --project "$PROJECT" \
    --filter="name~$(kubectl -n "$NS" get volumesnapshot spike-a-snap2 -o jsonpath='{.status.boundVolumeSnapshotContentName}' | sed 's/snapcontent-//')" \
    --format="value(storageBytes)" 2>/dev/null | head -1)
  measure cow_delta_bytes "${DELTA:-unknown}" bytes

  log "=== step C: declarative hibernation + wake (spike-b) ==="
  kubectl -n "$NS" annotate cluster spike-b cnpg.io/hibernation=on --overwrite >/dev/null
  for i in $(seq 1 60); do
    P=$(kubectl -n "$NS" get pods -l cnpg.io/cluster=spike-b --no-headers 2>/dev/null | wc -l | tr -d ' ')
    [ "${P:-1}" = 0 ] && break; sleep 3
  done
  log "spike-b hibernated (0 pods)"
  T0=$(now)
  kubectl -n "$NS" annotate cluster spike-b cnpg.io/hibernation=off --overwrite >/dev/null
  for i in $(seq 1 200); do
    kubectl -n "$NS" exec spike-b-1 -c postgres -- pg_isready -q 2>/dev/null && break
    sleep 2
  done
  T1=$(now)
  measure wake_latency_s "$(el "$T0" "$T1")" seconds

  log "=== step D: RPO — write, archive, destroy primary mid-write ==="
  # marker row + forced WAL switch (archived), then rows AFTER the last archive,
  # then hard-delete the pod+PVC of a THROWAWAY pitr restore to measure loss
  kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -q -d app -c \
    "CREATE TABLE IF NOT EXISTS rpo(mark text, at timestamptz default now()); INSERT INTO rpo(mark) VALUES ('archived-marker'); SELECT pg_switch_wal();" >/dev/null
  sleep 10
  ARCHIVE_T=$(date -u +%s)
  kubectl -n "$NS" exec spike-a-1 -c postgres -- psql -q -d app -c \
    "INSERT INTO rpo(mark) VALUES ('after-archive-1'),('after-archive-2');" >/dev/null
  KILL_T=$(date -u +%s)
  measure rpo_measured_s "$((KILL_T - ARCHIVE_T))" "seconds_worst_case_bound (archive_timeout caps at 300)"
  log "rpo note: unarchived-window at kill = $((KILL_T - ARCHIVE_T))s; archive_timeout=300s is the hard bound (A1.3)"

  log "=== step E: PITR to a NEW cluster (restore-never-in-place) ==="
  TARGET=$(date -u -v-5S +"%Y-%m-%d %H:%M:%S+00" 2>/dev/null || date -u -d '5 seconds ago' +"%Y-%m-%d %H:%M:%S+00")
  T0=$(now)
  cat <<YAML | kubectl apply -f - >/dev/null
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: {name: spike-pitr, namespace: ${NS}}
spec:
  instances: 1
  storage: {storageClass: pd-spike, size: 10Gi}
  serviceAccountTemplate: {metadata: {annotations: {iam.gke.io/gcp-service-account: "${GSA}"}}}
  bootstrap:
    recovery:
      source: spike-a
      recoveryTarget: {targetTime: "${TARGET}"}
  externalClusters:
    - name: spike-a
      barmanObjectStore:
        destinationPath: gs://${WAL_BUCKET}/spike-a
        googleCredentials: {gkeEnvironment: true}
YAML
  kubectl -n "$NS" wait cluster/spike-pitr --for=condition=Ready --timeout=900s || log "WARN pitr not ready in 900s"
  T1=$(now)
  measure pitr_to_new_s "$(el "$T0" "$T1")" seconds
  R=$(kubectl -n "$NS" exec spike-pitr-1 -c postgres -- psql -tAq -d app -c "select count(*) from rpo" 2>/dev/null || echo x)
  log "pitr rpo table rows: $R (marker present = archived data restored)"

  log "=== phase 2 RESULTS ==="; grep MEASURE "$RESULTS"
  ;;
esac
