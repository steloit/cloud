#!/usr/bin/env bash
# Shared config for the Steloit Product Build GitHub Project automation.
# Source of truth: claudedocs/implementation-plan.md — regenerate issues from data/*.json.
set -euo pipefail

export ORG="steloit"
export REPO="steloit/cloud"
export PROJECT_TITLE="Steloit Product Build"

# Sprint 0 ends Fri 2026-07-24; sprints 1-16 are 2 weeks each after that.
export SPRINT0_DUE="2026-07-24"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export DATA_DIR="$SCRIPT_DIR/data"
export STATE_DIR="$SCRIPT_DIR/.state"
mkdir -p "$STATE_DIR"

need() { command -v "$1" >/dev/null || { echo "missing dependency: $1" >&2; exit 1; }; }
need gh; need jq

preflight() {
  gh auth status >/dev/null || { echo "gh not authenticated" >&2; exit 1; }
  gh repo view "$REPO" --json name >/dev/null || { echo "cannot access $REPO" >&2; exit 1; }
  echo "preflight ok: $(gh api user --jq .login) → $REPO (org: $ORG)"
}

# due date for "Sprint N" / "M#" milestones (macOS date)
sprint_due() { # $1 = sprint number 0..16
  local n="$1"
  date -j -v+"$((n * 14))"d -f "%Y-%m-%d" "$SPRINT0_DUE" +%Y-%m-%d
}
