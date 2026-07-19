#!/usr/bin/env bash
# T1.0 spike — COMPLETE idempotent teardown for the PD-CSI path (review
# finding: the run scripts only tear down in-cluster state; this removes
# EVERYTHING billable or residual, is safe to re-run, and prints a final
# verification listing. Output belongs in results/teardown.log (evidence).
set -uo pipefail
cd "$(dirname "$0")"
PROJECT="${SPIKE_PROJECT_ID:-steloit-dev}" ZONE=us-central1-a
WAL_BUCKET=${PROJECT}-wal-customer
GSA=spike-wal@${PROJECT}.iam.gserviceaccount.com
log(){ echo "$(date -u +%FT%TZ) $*"; }

log "=== teardown: in-cluster (best-effort; clusters may already be gone) ==="
for C in spike-e2 spike-db spike-cnpg; do
  if gcloud container clusters describe "$C" --zone "$ZONE" --project "$PROJECT" >/dev/null 2>&1; then
    gcloud container clusters get-credentials "$C" --zone "$ZONE" --project "$PROJECT" >/dev/null 2>&1
    kubectl delete volumesnapshot -n spike --all --wait=false 2>/dev/null
    kubectl delete ns spike --wait=false 2>/dev/null
    # wait for PVC/PD reclaim before killing the cluster (a stuck finalizer
    # would orphan PDs that bill forever)
    for i in $(seq 1 30); do
      N=$(kubectl get pvc -n spike --no-headers 2>/dev/null | wc -l | tr -d ' ')
      [ "${N:-0}" = 0 ] && break; sleep 5
    done
    log "deleting cluster $C"
    gcloud container clusters delete "$C" --zone "$ZONE" --project "$PROJECT" --quiet 2>&1 | tail -1
  else
    log "cluster $C: already gone"
  fi
done

log "=== teardown: WAL/backup objects ==="
gcloud storage rm -r "gs://${WAL_BUCKET}/spike-a" 2>/dev/null && log "WAL objects removed" || log "WAL objects: already gone"

log "=== teardown: bucket IAM binding (live or tombstoned) ==="
POLICY=$(gcloud storage buckets get-iam-policy "gs://${WAL_BUCKET}" --format=json 2>/dev/null)
for M in "serviceAccount:${GSA}" $(echo "$POLICY" | grep -oE "deleted:serviceAccount:${GSA}\?uid=[0-9]+"); do
  gcloud storage buckets remove-iam-policy-binding "gs://${WAL_BUCKET}" \
    --member="$M" --role=roles/storage.objectAdmin >/dev/null 2>&1 && log "removed binding: $M"
done
T=$(gcloud storage buckets get-iam-policy "gs://${WAL_BUCKET}" --format=json 2>/dev/null | grep -c "spike-wal" || true)
log "bindings referencing spike-wal remaining: ${T:-0}"

log "=== teardown: temp GSA ==="
gcloud iam service-accounts delete "$GSA" --project "$PROJECT" --quiet 2>/dev/null && log "GSA deleted" || log "GSA: already gone"

log "=== teardown: orphan sweep (must all be empty) ==="
log "clusters:";  gcloud container clusters list --project "$PROJECT" --format="value(name,status)" 2>&1
log "instances:"; gcloud compute instances list --project "$PROJECT" --format="value(name,status)" 2>&1
log "disks:";     gcloud compute disks list --project "$PROJECT" --format="value(name,sizeGb)" 2>&1
log "snapshots:"; gcloud compute snapshots list --project "$PROJECT" --format="value(name)" 2>&1
log "teardown complete — the four listings above must be empty"
