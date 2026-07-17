#!/usr/bin/env bash
# Create Sprint 0-16 + M1-M8 milestones in $REPO. Idempotent (skips existing titles).
source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
preflight

existing="$(gh api "repos/$REPO/milestones?state=all&per_page=100" --jq '[.[].title]')"

create_ms() { # title, due (YYYY-MM-DD), description
  local title="$1" due="$2" desc="$3"
  if echo "$existing" | jq -e --arg t "$title" 'index($t)' >/dev/null; then
    echo "exists: $title"; return
  fi
  gh api "repos/$REPO/milestones" -f title="$title" -f due_on="${due}T23:59:59Z" -f description="$desc" --jq .number >/dev/null
  echo "created: $title (due $due)"
}

create_ms "Sprint 0" "$(sprint_due 0)" "Setup & spec rulings (P1-P5, S1-S7). Exit: Sprint 1 unblocked."
for n in $(seq 1 16); do
  case $n in
    1) d="E1: cell-0, Neon-GCS spike + go/no-go ADR";;
    2) d="E1 finish + E2 start: reconciler v0, control-plane DB, users/orgs/sessions";;
    3) d="E2 finish + E3/E5 start: RBAC, events, projects/envs, CLI skeleton";;
    4) d="E3 finish + E4 start: estimate engine, Postgres provisioning e2e, bindings";;
    5) d="E4 + E6: build pipeline, deploy+rollback, metrics/logs baseline, metering";;
    6) d="E4 finish: PR->branched preview->teardown; E8-lite; alpha hardening. M4.";;
    7) d="E7 + E8-2: MFA/sessions/org keys; console A/G planes live; docs";;
    8) d="E9-1/2 + E8-3: Valkey, object storage, queue review ADR, TS SDK v0";;
    9) d="E9-3 + E10 + E8-4: queue impl, alert evaluator+backtest, custom domains. M5.";;
    10) d="E10 + E11 start + E8-5: notifications, emails, pricing/subscriptions. M6.";;
    11) d="E11 + E12 + E8-6: quotas, invoices, policies; console settings live";;
    12) d="E11 + E12: payment provider, dunning, templates, plan change/cancel";;
    13) d="E11 finish + E13 start: first charge (M7), dashboards, AI tools";;
    14) d="E13 + E8-7: threads/insights/proposals; console AI live; knob-turns";;
    15) d="E13 finish + E14-W1: AI polish, data-plane read surfaces";;
    16) d="E14 + hardening: spec closure, drills, security review, GA review. M8.";;
  esac
  create_ms "Sprint $n" "$(sprint_due "$n")" "$d"
done

# M1-M8 checkpoints (due = their sprint's end)
create_ms "M1 — Cell-0 alive" "$(sprint_due 1)"  "Spike ADR recorded; terraform apply from empty works; checkpoint: spike fails both fallbacks -> stop, amend D3"
create_ms "M2 — Identity real" "$(sprint_due 3)"  "Identity + RBAC + audit real; QA scenarios 6 & 8 green"
create_ms "M3 — Estimate-gated provisioning" "$(sprint_due 4)"  "steloit db create -> estimate -> approved -> ready, metered"
create_ms "M4 — Alpha live" "$(sprint_due 6)"  "Full alpha path live; five design partners onboard"
create_ms "M5 — Console live" "$(sprint_due 9)"  "Console fully usable for the alpha path against the real API"
create_ms "M6 — Data layer complete" "$(sprint_due 10)" "Valkey + object storage + queue shipped"
create_ms "M7 — First payment" "$(sprint_due 13)" "First payment clears; hiring unfreeze; capacity knob-turns; Mumbai"
create_ms "M8 — v1 GA-ready" "$(sprint_due 16)" "Regression checklist green; security review done; abuse controls full"

echo "milestones done"
