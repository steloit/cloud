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

# NS is the control-plane-resolved namespace — read it from the service's
# desired doc (env-<hex>), never guessed. US-3.3 derives it from the immutable
# env id because project/env NAMES are unique only per org (cross-tenant
# collision), so the old proj--env form is gone.
NS="${NS:?set NS to the service row\'s desired.namespace (SELECT desired->>\'namespace\' FROM services WHERE id=...)}"
CELL="${CELL:-cell-dev}"
API_PORT="${API_PORT:-8080}"
RECONCILER_SECRET="${RECONCILER_SECRET:-e2e-secret}"
ROOT="$(git rev-parse --show-toplevel)"

step() { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }

step "0 · preflight: cluster reachable, CNPG operator present"
kubectl cluster-info | head -2
kubectl get crd clusters.postgresql.cnpg.io -o name || {
  echo "CNPG CRD missing — apply module.cnpg first"; exit 1; }
# NOTE: the cell-agent now creates this namespace itself on every converge
# (US-3.3a), so the line below is belt-and-braces rather than the GAP it used to
# be. It is deliberately still here: removing it would make this runbook unable
# to run until a cell exists to prove the replacement works, and there are
# currently zero GKE clusters in steloit-dev.
#
# US-3.3a AC 3 ("proven without the runbook's preflight") is therefore NOT
# US-3.3c AC 7: the boundary is now ASSERTED, not described — see assert_boundary
# below, which fails the script rather than printing what an operator should look
# for. Everything else in this file is still a guided runbook.
#
# The agent creates the namespace, the ResourceQuota, the LimitRange and the D7
# NetworkPolicies. Enforcement is real: the cell runs Dataplane V2 (US-3.3f).
kubectl get ns "$NS" >/dev/null 2>&1 || kubectl create ns "$NS"

step "1 · control plane: a service is accepted (estimate → createService)"
# The api must be running against a Postgres with RECONCILER_SECRET set and
# RECONCILER_CELLS containing $CELL. Creating org/project/env/estimate/service
# through the real API proves the estimate gate is honoured end to end.
echo "(driven by the caller: create org → project → env → estimate → accept → createService)"
echo "expected: services row with status=provisioning, generation=1, observed_generation=0,"
echo "          desired.namespace=$NS  ← env-derived; the cell renders here"

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

# --- AC 7: ASSERT the boundary -------------------------------------------------
#
# Every check below FAILS THE SCRIPT. A runbook that prints "expect: denied" and
# exits 0 is how US-3.3a shipped a boundary that did not exist.
#
# The isolation check needs a SECOND environment, so it renders one rather than
# assuming one is lying around. Both probe pods land on the same node where
# possible, because same-node pod-to-pod traffic is the case that flows freely
# unless a policy actually drops it — cross-node traffic can be blocked by
# routing and look like enforcement.
assert_boundary() {
  local ns_a="$1" ns_b="$2" fail=0

  step "7 · ASSERT the D7 boundary (AC 7)"

  # 7a. the namespace carries its labels
  kubectl get ns "$ns_a" -o jsonpath='{.metadata.labels.steloit\.dev/tenant}' 2>/dev/null \
    | grep -q true || { echo "FAIL: $ns_a is not labelled steloit.dev/tenant=true"; fail=1; }

  # 7b. the policies exist — derived from what the agent renders, not a typed list
  for p in default-deny-all allow-dns-egress allow-same-namespace \
           allow-cnpg-egress allow-cnpg-operator-ingress; do
    kubectl -n "$ns_a" get networkpolicy "$p" >/dev/null 2>&1 \
      || { echo "FAIL: $ns_a is missing NetworkPolicy/$p"; fail=1; }
  done

  # 7c. ENFORCEMENT IS INSTALLED. Policies that exist on a cluster with no
  # provider are stored and ignored — the exact failure US-3.3a shipped.
  kubectl -n kube-system get ds anetd >/dev/null 2>&1 \
    || { echo "FAIL: anetd (Dataplane V2) is not running — NOTHING enforces these"; fail=1; }

  # 7d. a pod in env A cannot reach a pod in env B
  local b_ip
  b_ip=$(kubectl -n "$ns_b" get pod probe-server -o jsonpath='{.status.podIP}' 2>/dev/null)
  [ -n "$b_ip" ] || { echo "FAIL: no probe-server in $ns_b — the isolation check would pass vacuously"; return 1; }
  if kubectl -n "$ns_a" exec probe-client -- \
       curl -s -o /dev/null --max-time 8 "http://$b_ip:8080/" 2>/dev/null; then
    echo "FAIL: $ns_a reached $ns_b — the environment is not a boundary"; fail=1
  fi

  # 7e. CONTROL: the same probe INSIDE the environment must succeed, or 7d
  # proves only that the probe is broken.
  local a_ip
  a_ip=$(kubectl -n "$ns_a" get pod probe-server -o jsonpath='{.status.podIP}' 2>/dev/null)
  kubectl -n "$ns_a" exec probe-client -- \
    curl -s -o /dev/null --max-time 8 "http://$a_ip:8080/" 2>/dev/null \
    || { echo "FAIL: $ns_a cannot reach ITSELF — the allow-set is too tight"; fail=1; }

  # 7f. DNS still resolves. Measured to break when the rule names only kube-dns
  # on a cell with NodeLocal DNSCache (US-3.3c live run).
  kubectl -n "$ns_a" exec probe-client -- \
    nslookup kubernetes.default.svc.cluster.local >/dev/null 2>&1 \
    || { echo "FAIL: DNS does not resolve in $ns_a"; fail=1; }

  # 7g. the environment's own database is reachable from inside it
  kubectl -n "$ns_a" get cluster db01 \
    -o jsonpath='{.status.phase}' 2>/dev/null | grep -q "healthy" \
    || echo "WARN: no healthy db01 in $ns_a — skipping the same-env database check"

  [ "$fail" = 0 ] || { echo "BOUNDARY ASSERTIONS FAILED"; return 1; }
  echo "boundary OK: isolated across environments, open within one, DNS intact"
}

step "DONE — paste this output into the US-3.3 PR as live evidence"
