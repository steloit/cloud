#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Step 6 — declarative hibernation both clusters; timed wake of spike-b.
# Cold AND warm runs (common-mistakes rule: cold-only is not the number).
for c in spike-a spike-b; do
  kubectl -n "$NS" annotate cluster "$c" cnpg.io/hibernation=on --overwrite
done
kubectl -n "$NS" wait cluster/spike-b --for=condition=Ready=false --timeout=300s || true

for run in cold warm; do
  kubectl -n "$NS" annotate cluster spike-b cnpg.io/hibernation=off --overwrite
  T0=$(now)
  kubectl -n "$NS" wait cluster/spike-b --for=condition=Ready --timeout=600s
  T1=$(now)
  measure "wake_latency_${run}_s" "$(elapsed "$T0" "$T1")" seconds
  [ "$run" = cold ] && { kubectl -n "$NS" annotate cluster spike-b cnpg.io/hibernation=on --overwrite; sleep 30; }
done
# canonical key = cold (the honest number for the gateway design)
grep 'wake_latency_cold_s' logs/spike.log | tail -1 | sed 's/wake_latency_cold_s/wake_latency_s/' | tee -a logs/spike.log
