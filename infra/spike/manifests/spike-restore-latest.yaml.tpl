# Restore-to-latest (deterministic; the committed reproducible path that
# produced pitr_to_new_s). targetTime PITR is a documented sharp edge (F4):
# a target beyond archived WAL leaves recovery waiting for future segments.
# __GSA__ substituted by run-pdcsi-phase2.sh.
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: {name: spike-pitr, namespace: spike}
spec:
  instances: 1
  storage: {storageClass: pd-spike, size: 10Gi}
  serviceAccountTemplate: {metadata: {annotations: {iam.gke.io/gcp-service-account: "__GSA__"}}}
  bootstrap:
    recovery:
      source: spike-a
  externalClusters:
    - name: spike-a
      barmanObjectStore:
        destinationPath: gs://__WAL_BUCKET__/spike-a
        googleCredentials: {gkeEnvironment: true}
