#!/usr/bin/env bash
# Re-verify docs/founder-config.md §2 against the live project. READ-ONLY.
#
# WHY (T1.9/T1.10). §2 asserted eight resources as created that do not exist, for
# weeks, and other tasks cited it as fact. The correction added a "last verified"
# DATE — which is not a mechanism. This is the mechanism's first half: make
# re-verifying cheap enough that it happens. It reports; it never edits.
#
# It also exists because T1.9's evidence lived as loose commands in a task body and
# was found INCOMPLETE twice — once by review (the KMS key was claimed unchecked
# because the command was never run) and once by me (the state-file and API-count
# measurements had no recorded command at all).
#
#   ./scripts/infra/verify-live.sh [project]      default: steloit-dev
set -euo pipefail
PROJECT="${1:-steloit-dev}"
export CLOUDSDK_CORE_PROJECT="$PROJECT"

if ! gcloud auth print-access-token >/dev/null 2>&1; then
  echo "verify-live: credentials are not usable — run 'gcloud auth login' first." >&2
  echo "Do NOT record a verification date without running this." >&2
  exit 2
fi
echo "# verify-live: $PROJECT as $(gcloud config get-value account 2>/dev/null)"
echo "# $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo

row() { printf '%-28s %s\n' "$1" "$2"; }
list() { gcloud "$@" 2>/dev/null | tr '\n' ' ' | sed 's/ *$//'; }

echo "## Resources §2 claims"
row "buckets"          "$(list storage buckets list --format=value\(name\))"
row "artifact repos"   "$(list artifacts repositories list --format=value\(name\))"
row "service accounts" "$(list iam service-accounts list --format=value\(email\))"
row "WIF pools"        "$(list iam workload-identity-pools list --location=global --format=value\(name\))"
row "KMS key rings"    "$(list kms keyrings list --location=us-central1 --format=value\(name\))"
row "KMS keys"         "$(list kms keys list --location=us-central1 --keyring=cell-dev-core --format=value\(name\))"
row "APIs enabled"     "$(gcloud services list --enabled --format='value(config.name)' 2>/dev/null | wc -l | tr -d ' ') total"
echo

echo "## Terraform state — what is TRACKED (an untracked resource re-applies as already-exists)"
for obj in $(gcloud storage ls -r "gs://${PROJECT}-tfstate/**" 2>/dev/null | grep '\.tfstate$' || true); do
  echo "  $obj"
  gcloud storage cat "$obj" 2>/dev/null | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('    serial', d.get('serial'), '·', len(d.get('resources',[])), 'resource(s)')
for r in d.get('resources',[]):
    print('     ', r.get('module','root'), r['type'] + '.' + r['name'])"
done
echo

echo "## Cost — anything running?"
row "GKE clusters"     "$(list container clusters list --format=value\(name\))"
row "compute instances" "$(list compute instances list --format=value\(name\))"
row "persistent disks" "$(list compute disks list --format=value\(name\))"
echo
echo "# Compare against docs/founder-config.md §2. A row that disagrees is a finding,"
echo "# not a licence to edit the live project."
