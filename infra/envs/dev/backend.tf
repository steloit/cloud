terraform {
  # Bucket/prefix supplied via -backend-config (infra/README.md bootstrap);
  # keeping this empty lets `init -backend=false` validate with no credentials.
  backend "gcs" {}
}
