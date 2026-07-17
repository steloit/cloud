#!/usr/bin/env bash
# Add every created issue to the ProjectV2 and set custom fields.
# Requires 03-project.sh to have run (needs `project` token scope).
source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
preflight

MAP="$STATE_DIR/issue-map.json"
pnum="$(cat "$STATE_DIR/project-number")"
PID="$(gh project list --owner "$ORG" --format json --limit 100 | jq -r --argjson n "$pnum" '.projects[] | select(.number==$n) | .id')"

fields="$(gh project field-list "$pnum" --owner "$ORG" --format json --limit 100)"
fid() { jq -r --arg n "$1" '.fields[] | select(.name==$n) | .id' <<<"$fields"; }
oid() { jq -r --arg n "$1" --arg o "$2" '.fields[] | select(.name==$n) | .options[] | select(.name==$o) | .id' <<<"$fields"; }

F_SPRINT="$(fid Sprint)"; F_EPIC="$(fid Epic)"; F_MODULE="$(fid Module)"
F_PRIO="$(fid Priority)"; F_EST="$(fid Estimate)"; F_PHASE="$(fid Phase)"

ITEMS="$STATE_DIR/item-map.json"
[ -f "$ITEMS" ] || echo '{}' > "$ITEMS"
item_id() { jq -r --arg k "$1" '.[$k] // empty' "$ITEMS"; }
save_item() { jq --arg k "$1" --arg v "$2" '.[$k]=$v' "$ITEMS" > "$ITEMS.tmp" && mv "$ITEMS.tmp" "$ITEMS"; }

set_select() { # item, field-id, field-name, option-name
  local item="$1" f="$2" fname="$3" opt="$4"
  [ -n "$opt" ] && [ "$opt" != "null" ] && [ "$opt" != "—" ] || return 0
  local o; o="$(oid "$fname" "$opt")"
  [ -n "$o" ] || { echo "  ! no option '$opt' for $fname"; return 0; }
  gh project item-edit --id "$item" --project-id "$PID" --field-id "$f" --single-select-option-id "$o" >/dev/null
}

process() { # one json row (epic or child); epics set Epic=own key
  local row="$1"
  local key sprint epic module prio est phase n item url
  key="$(jq -r .key <<<"$row")"
  n="$(jq -r --arg k "$key" '.[$k] // empty' "$MAP")"
  [ -n "$n" ] || { echo "skip (no issue): $key"; return; }
  item="$(item_id "$key")"
  if [ -z "$item" ]; then
    url="https://github.com/$REPO/issues/$n"
    item="$(gh project item-add "$pnum" --owner "$ORG" --url "$url" --format json | jq -r .id)"
    save_item "$key" "$item"
  fi
  sprint="$(jq -r '.sprint // .milestone' <<<"$row")"
  epic="$(jq -r '.epic // .key' <<<"$row")"
  module="$(jq -r '.module // empty' <<<"$row")"
  prio="$(jq -r '.priority // empty' <<<"$row")"
  est="$(jq -r '.estimate // empty' <<<"$row")"
  phase="$(jq -r '.phase // empty' <<<"$row")"
  case "$sprint" in Sprint*) : ;; *) sprint="" ;; esac   # M1-M8 checkpoints carry .sprint explicitly
  [ -n "$sprint" ] && set_select "$item" "$F_SPRINT" Sprint "$sprint"
  set_select "$item" "$F_EPIC" Epic "$epic"
  set_select "$item" "$F_MODULE" Module "$module"
  set_select "$item" "$F_PRIO" Priority "$prio"
  set_select "$item" "$F_PHASE" Phase "$phase"
  if [ -n "$est" ] && [ "$est" != "null" ]; then
    gh project item-edit --id "$item" --project-id "$PID" --field-id "$F_EST" --number "$est" >/dev/null
  fi
  echo "fields set: $key (#$n)"
  sleep 0.2
}

for f in "$DATA_DIR/epics.json" "$DATA_DIR/issues.json"; do
  jq -c '.[]' "$f" | while read -r row; do process "$row"; done
done

echo "project items done"
