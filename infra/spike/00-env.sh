#!/usr/bin/env bash
# Shared spike configuration. GATE: requires P1 — export SPIKE_PROJECT_ID first.
set -euo pipefail

: "${SPIKE_PROJECT_ID:?export SPIKE_PROJECT_ID=<steloit-dev project id> (P1 gate)}"
export ZONE="${SPIKE_ZONE:-us-central1-a}"
export CLUSTER="${SPIKE_CLUSTER:-cell-dev}"
export NS="spike"
export CNPG_VERSION="1.30.0"          # architecture §3 pin (<=1.30, in-tree Barman)
export ZFS_LOCALPV_VERSION="2.6.0"    # OpenEBS ZFS-LocalPV chart
export WAL_BUCKET="${SPIKE_PROJECT_ID}-wal-customer"

mkdir -p logs
log() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "logs/spike.log"; }
measure() { # measure <key> <value> <unit>
  log "MEASURE $1=$2 unit=$3"
}
# monotonic seconds with millis
now() { python3 -c 'import time; print(f"{time.monotonic():.3f}")'; }
elapsed() { python3 -c "print(f'{$2 - $1:.1f}')"; }

kubectl_ctx() {
  gcloud container clusters get-credentials "$CLUSTER" --zone "$ZONE" --project "$SPIKE_PROJECT_ID" >/dev/null
}
