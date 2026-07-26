#!/usr/bin/env bash
# Create/refresh all labels in $REPO. Idempotent (--force updates color/description).
source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
preflight

labels=(
  "epic|3E4B9E|Epic tracking issue (E0-E14, EQA, EOPS)"
  "story|1D76DB|User story with acceptance criteria"
  "task|0E8A16|Technical task"
  "milestone-checkpoint|5319E7|M1-M8 exit-criteria checkpoint"
  "spec-ruling|D93F0B|Founder decision needed (S-process)"
  "setup|FBCA04|Sprint 0 setup item"
  "blocked|B60205|Has unresolved blockers"
  "Backend|0052CC|Control-plane services & API"
  "Frontend|006B75|Console integration"
  "Infrastructure|C5DEF5|GCP / Terraform / cluster"
  "Platform|BFD4F2|Data plane, CNPG substrate, drivers, reconciler"
  "CLI|E4E669|steloit CLI & SDK"
  "API|FEF2C0|openapi.yaml contract work"
  "Database|D4C5F9|Schema, migrations, data model"
  "QA|F9D0C4|Tests, invariants, drills"
  "DevOps|BFDADC|CI/CD, environments, ops"
  "AI|7057FF|Assistant plane (four laws)"
  "Billing|0E8A16|Metering, quotas, plans, invoices"
  "Security|B60205|AuthN/Z, isolation, abuse controls"
  "Documentation|0075CA|Docs, references, onboarding guides"
)

for l in "${labels[@]}"; do
  IFS='|' read -r name color desc <<<"$l"
  gh label create "$name" -R "$REPO" --color "$color" --description "$desc" --force >/dev/null \
    && echo "label: $name"
done
echo "labels done (${#labels[@]})"
