#!/usr/bin/env bash
# US-3.3 end-to-end: an accepted service provisions via the reconciler and
# reaches ready on a REAL cell, with metering opening at ready.
#
# This is the headline AC's evidence script. It is deliberately a runbook, not a
# Go test: it spans two processes (api + cell-agent) and a live cluster, so it is
# run and its output pasted, per the founder's "live/manual evidence" ruling.
#
# Prereqs: the dev cell applied (infra/envs/dev), kubectl context on it, a
# Postgres for the control plane, and gcloud auth as the founder account.
set -euo pipefail

NS="${NS:-e2e--prod}"
CELL="${CELL:-cell-dev}"
API_PORT="${API_PORT:-8080}"
RECONCILER_SECRET="${RECONCILER_SECRET:-e2e-secret}"
ROOT="$(git rev-parse --show-toplevel)"

step() { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }

step "0 · preflight: cluster reachable, CNPG operator present"
kubectl cluster-info | head -2
kubectl get crd clusters.postgresql.cnpg.io -o name || {
  echo "CNPG CRD missing — apply module.cnpg first"; exit 1; }
kubectl get ns "$NS" >/dev/null 2>&1 || kubectl create ns "$NS"

step "1 · control plane: a service is accepted (estimate → createService)"
# The api must be running against a Postgres with RECONCILER_SECRET set and
# RECONCILER_CELLS containing $CELL. Creating org/project/env/estimate/service
# through the real API proves the estimate gate is honoured end to end.
echo "(driven by the caller: create org → project → env → estimate → accept → createService)"
echo "expected: services row with status=provisioning, generation=1, observed_generation=0,"
echo "          desired.namespace=$NS  ← the cell renders here (US-3.3 Step 1)"

step "2 · the agent converges it onto the cell"
echo "run, in the cluster (or locally with a kubeconfig):"
cat <<EOF
  CONTROL_PLANE_URL=http://<api>:$API_PORT \\
  RECONCILER_SECRET=$RECONCILER_SECRET \\
  RECONCILER_CELL=$CELL \\
  CELL_GSA_EMAIL=<cell customer GSA> \\
  CELL_WAL_BUCKET=steloit-dev-wal-customer \\
  ./cell-agent
EOF
echo "expected log: 'renderer: CNPG (in-cluster, real apply)' — NOT the Ack fallback"

step "3 · evidence: the CNPG cluster exists in the env namespace"
kubectl -n "$NS" get clusters.postgresql.cnpg.io -o wide || true
echo "--- the applied manifest (should match T3.4's golden shape) ---"
kubectl -n "$NS" get clusters.postgresql.cnpg.io -o yaml 2>/dev/null | \
  grep -E "archive_timeout|storageClass|pool:|gkeEnvironment|retentionPolicy|instances:" || true
echo "--- the F3 base backup (WAL alone is not restorable, ADR-0007) ---"
kubectl -n "$NS" get scheduledbackups.postgresql.cnpg.io -o wide || true

step "4 · evidence: the cluster reaches ready"
kubectl -n "$NS" wait --for=condition=Ready cluster.postgresql.cnpg.io --all --timeout=600s || true
kubectl -n "$NS" get clusters.postgresql.cnpg.io \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\n"}{end}'

step "5 · evidence: the control plane observed ready and metering opened"
cat <<'EOF'
  -- against the control-plane Postgres:
  SELECT id, status, generation, observed_generation, last_reconciled_at
    FROM services WHERE cell_id = 'cell-dev';
  -- expect: status=ready, observed_generation = generation

  SELECT action, subject FROM events
   WHERE action = 'service.ready' ORDER BY at DESC LIMIT 5;
  -- expect: exactly one service.ready per service (D10 spine event)

  SELECT service_id, edge, product, rate_cents, at FROM usage_events
   ORDER BY at DESC LIMIT 5;
  -- expect: exactly one edge='open' per service, stamped at the ready
  -- transition — and NOTHING before it (D10: metering starts at ready)
EOF

step "6 · teardown reaches the cell"
echo "DELETE the service, then:"
echo "  kubectl -n $NS get clusters.postgresql.cnpg.io   # expect: gone"

step "DONE — paste this output into the US-3.3 PR as live evidence"
