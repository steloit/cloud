#!/usr/bin/env bash
# Create the org-level ProjectV2 + custom fields. Requires the `project` token scope:
#   gh auth refresh -h github.com -s project,read:project
# Idempotent: reuses an existing project with the same title.
source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
preflight

# --- find or create the project ---
pnum="$(gh project list --owner "$ORG" --format json --limit 100 \
  | jq -r --arg t "$PROJECT_TITLE" '.projects[] | select(.title==$t) | .number' | head -1)"
if [ -z "$pnum" ]; then
  pnum="$(gh project create --owner "$ORG" --title "$PROJECT_TITLE" --format json | jq -r .number)"
  echo "created project #$pnum"
else
  echo "project exists: #$pnum"
fi
echo "$pnum" > "$STATE_DIR/project-number"

gh project link "$pnum" --owner "$ORG" --repo "$REPO" 2>/dev/null || true

# --- custom fields (Status is built-in) ---
fields_json="$(gh project field-list "$pnum" --owner "$ORG" --format json --limit 100)"
have_field() { echo "$fields_json" | jq -e --arg n "$1" '.fields[] | select(.name==$n)' >/dev/null; }

mkfield() { # name, type, [options csv]
  local name="$1" type="$2" opts="${3:-}"
  if have_field "$name"; then echo "field exists: $name"; return; fi
  if [ "$type" = "SINGLE_SELECT" ]; then
    gh project field-create "$pnum" --owner "$ORG" --name "$name" --data-type SINGLE_SELECT \
      --single-select-options "$opts" >/dev/null
  else
    gh project field-create "$pnum" --owner "$ORG" --name "$name" --data-type "$type" >/dev/null
  fi
  echo "field created: $name"
}

sprints="Sprint 0"; for n in $(seq 1 16); do sprints="$sprints,Sprint $n"; done
epics="E0,E1,E2,E3,E4,E5,E6,E7,E8,E9,E10,E11,E12,E13,E14,EQA,EOPS"
modules="M1 Substrate,M2 Identity,M3 Control plane,M4 Provisioning,M5 Deploy,M6 Observe,M7 Billing,M8 Governance,M9 AI,M10 Clients,M11 Comms,M12 Data-plane depth,Cross-cutting"

mkfield "Sprint"   SINGLE_SELECT "$sprints"
mkfield "Epic"     SINGLE_SELECT "$epics"
mkfield "Module"   SINGLE_SELECT "$modules"
mkfield "Priority" SINGLE_SELECT "Critical,High,Medium,Low"
mkfield "Estimate" NUMBER
mkfield "Phase"    SINGLE_SELECT "MVP,V1,Future"
mkfield "Owner"    TEXT

echo "project + fields done (project #$pnum)"
