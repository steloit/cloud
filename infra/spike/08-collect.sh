#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Assemble measurements.txt from every MEASURE line (latest value per key wins).
grep -h 'MEASURE ' logs/spike.log | awk '{for(i=1;i<=NF;i++) if($i ~ /^MEASURE$/){print $(i+1), $(i+2)}}' \
  | awk -F'[= ]' '{v[$1]=$0} END {for (k in v) print v[k]}' | sort | tee measurements.txt
log "collected $(wc -l < measurements.txt) measurements"
