#!/usr/bin/env bash
# T1.0 verify: passes only when the spike has RUN — findings ADR present with
# all six MEASURE keys + a Verdict, and every manifest parses.
set -euo pipefail
cd "$(dirname "$0")"

ADR="../../docs/adr/0007-substrate-spike-findings.md"
fail() { echo "check FAIL: $*" >&2; exit 1; }

[ -f "$ADR" ] || fail "findings ADR missing ($ADR) — the spike has not run (gate: P1)"
for key in snapshot_create_s branch_create_s cow_delta_bytes wake_latency_s rpo_measured_s pitr_to_new_s; do
  grep -q "$key" "$ADR" || fail "ADR missing measurement: $key"
done
grep -q '^## Verdict' "$ADR" || fail "ADR missing '## Verdict' section"

python3 - <<'PY'
import glob, sys, yaml
for f in glob.glob("manifests/*.yaml"):
    list(yaml.safe_load_all(open(f)))
print(f"manifests OK ({len(glob.glob('manifests/*.yaml'))} files)")
PY
echo "check PASS: spike findings complete"
