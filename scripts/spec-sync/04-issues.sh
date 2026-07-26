#!/usr/bin/env bash
# Create all issues from data/epics.json + data/issues.json in $REPO.
# Idempotent via .state/issue-map.json (key -> issue number); safe to re-run.
# Passes: 1) epics  2) children  3) sub-issue links  4) dependencies  5) epic checklists
source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
preflight

MAP="$STATE_DIR/issue-map.json"
[ -f "$MAP" ] || echo '{}' > "$MAP"

key_num() { jq -r --arg k "$1" '.[$k] // empty' "$MAP"; }
save_num() { jq --arg k "$1" --argjson n "$2" '.[$k]=$n' "$MAP" > "$MAP.tmp" && mv "$MAP.tmp" "$MAP"; }

create_issue() { # key title labels_csv milestone body
  local key="$1" title="$2" labels="$3" milestone="$4" body="$5"
  local existing; existing="$(key_num "$key")"
  if [ -n "$existing" ]; then echo "exists: $key -> #$existing"; return; fi
  local args=(-R "$REPO" --title "$title" --body "$body")
  IFS=',' read -ra ls <<<"$labels"
  for l in "${ls[@]}"; do [ -n "$l" ] && args+=(--label "$l"); done
  [ -n "$milestone" ] && args+=(--milestone "$milestone")
  local url num
  url="$(gh issue create "${args[@]}")"
  num="${url##*/}"
  save_num "$key" "$num"
  echo "created: $key -> #$num"
  sleep 0.3   # stay clear of secondary rate limits
}

# ---- pass 1: epics ----
echo "== pass 1: epics =="
jq -c '.[]' "$DATA_DIR/epics.json" | while read -r e; do
  key="$(jq -r .key <<<"$e")"; title="$(jq -r .title <<<"$e")"
  labels="$(jq -r '.labels | join(",")' <<<"$e")"; ms="$(jq -r .milestone <<<"$e")"
  body="$(jq -r .body <<<"$e")"
  meta="**Epic $key** · Phase: $(jq -r .phase <<<"$e") · Module: $(jq -r .module <<<"$e") · Estimate: $(jq -r .estimate <<<"$e") EW · Priority: $(jq -r .priority <<<"$e")"
  create_issue "$key" "$title" "epic,$labels" "$ms" "$meta"$'\n\n'"$body"
done

# ---- pass 2: children ----
echo "== pass 2: children =="
jq -c '.[]' "$DATA_DIR/issues.json" | while read -r i; do
  key="$(jq -r .key <<<"$i")"; title="$(jq -r .title <<<"$i")"
  labels="$(jq -r '.labels | join(",")' <<<"$i")"; ms="$(jq -r .milestone <<<"$i")"
  epic="$(jq -r .epic <<<"$i")"; enum="$(key_num "$epic")"
  body="$(jq -r .body <<<"$i")"
  meta="Epic: $epic (#${enum:-?}) · Sprint: $(jq -r .sprint <<<"$i") · Phase: $(jq -r .phase <<<"$i") · Module: $(jq -r .module <<<"$i") · Estimate: $(jq -r .estimate <<<"$i") EW · Priority: $(jq -r .priority <<<"$i")"
  create_issue "$key" "$title" "$labels" "$ms" "$meta"$'\n\n'"$body"
done

# ---- pass 3: sub-issue links (best effort; API needs the child's database id) ----
echo "== pass 3: sub-issue links =="
jq -c '.[]' "$DATA_DIR/issues.json" | while read -r i; do
  key="$(jq -r .key <<<"$i")"; epic="$(jq -r .epic <<<"$i")"
  n="$(key_num "$key")"; en="$(key_num "$epic")"
  [ -n "$n" ] && [ -n "$en" ] || continue
  cid="$(gh api "repos/$REPO/issues/$n" --jq .id)"
  gh api -X POST "repos/$REPO/issues/$en/sub_issues" -F sub_issue_id="$cid" >/dev/null 2>&1 \
    && echo "linked: $key -> $epic" || true
  sleep 0.2
done

# ---- pass 4: dependencies (comment; marks blocked label) ----
echo "== pass 4: dependencies =="
DEPS_DONE="$STATE_DIR/deps-done"
touch "$DEPS_DONE"
jq -c '.[] | select(.deps and (.deps|length>0))' "$DATA_DIR/issues.json" "$DATA_DIR/epics.json" | while read -r i; do
  key="$(jq -r .key <<<"$i")"; n="$(key_num "$key")"
  [ -n "$n" ] || continue
  grep -qx "$key" "$DEPS_DONE" && continue
  refs=""
  for d in $(jq -r '.deps[]' <<<"$i"); do
    dn="$(key_num "$d")"; [ -n "$dn" ] && refs="$refs #$dn ($d)"
  done
  [ -n "$refs" ] || continue
  gh issue comment "$n" -R "$REPO" --body "⛔ **Blocked by:**$refs" >/dev/null
  gh issue edit "$n" -R "$REPO" --add-label blocked >/dev/null 2>&1 || true
  echo "$key" >> "$DEPS_DONE"
  echo "deps: $key ->$refs"
  sleep 0.3
done

# ---- pass 5: epic checklists ----
echo "== pass 5: epic checklists =="
jq -r '.[].key' "$DATA_DIR/epics.json" | while read -r ek; do
  en="$(key_num "$ek")"; [ -n "$en" ] || continue
  list="$(jq -r --arg e "$ek" '.[] | select(.epic==$e) | .key' "$DATA_DIR/issues.json")"
  [ -n "$list" ] || continue
  checklist="## Work items"$'\n'
  for k in $list; do
    kn="$(key_num "$k")"; [ -n "$kn" ] && checklist="$checklist- [ ] #$kn"$'\n'
  done
  current="$(gh issue view "$en" -R "$REPO" --json body --jq .body)"
  base="${current%%$'\n'## Work items*}"
  gh issue edit "$en" -R "$REPO" --body "$base"$'\n'"$checklist" >/dev/null
  echo "checklist: $ek (#$en)"
  sleep 0.3
done

echo "issues done: $(jq 'length' "$MAP") tracked in $MAP"
