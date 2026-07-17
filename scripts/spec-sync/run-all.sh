#!/usr/bin/env bash
# Recreate the entire Steloit Product Build project from scratch.
#   ./run-all.sh            # labels + milestones + issues (repo scope only)
#   ./run-all.sh --project  # additionally: ProjectV2 + fields + items
#                           # (requires: gh auth refresh -h github.com -s project,read:project)
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

bash 01-labels.sh
bash 02-milestones.sh
bash 04-issues.sh

if [ "${1:-}" = "--project" ]; then
  bash 03-project.sh
  bash 05-project-items.sh
fi

echo "done. Source of truth: claudedocs/implementation-plan.md"
