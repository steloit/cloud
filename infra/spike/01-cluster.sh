#!/usr/bin/env bash
# T1.0 substrate spike — e1-substrate-design.md §3. Rerunnable; every
# measurement prints a greppable "MEASURE key=value unit=" line and is
# appended to logs/ for 08-collect.sh.
set -euo pipefail
cd "$(dirname "$0")"
source ./00-env.sh

# Step 1 — platform-on-cluster: ZFS-LocalPV + snapshot classes + CNPG operator.
# The GKE cluster itself comes from `infra/envs/dev` (T1.1) — applied first.
kubectl_ctx
kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f -

helm repo add openebs-zfslocalpv https://openebs.github.io/zfs-localpv >/dev/null
helm repo update >/dev/null
helm upgrade --install zfs-localpv openebs-zfslocalpv/zfs-localpv \
  --namespace openebs --create-namespace \
  --version "$ZFS_LOCALPV_VERSION" --wait

kubectl apply -f manifests/storageclass.yaml
kubectl apply -f manifests/snapshotclass.yaml

kubectl apply --server-side -f \
  "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.30/releases/cnpg-${CNPG_VERSION}.yaml"
kubectl -n cnpg-system rollout status deployment/cnpg-controller-manager --timeout=300s
log "step 1 complete: zfs-localpv ${ZFS_LOCALPV_VERSION} + cnpg ${CNPG_VERSION} ready"
