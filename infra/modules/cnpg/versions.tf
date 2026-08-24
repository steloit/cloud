terraform {
  required_version = ">= 1.9"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.13"
    }
    kubectl = {
      # alekc/kubectl, NOT gavinbunney/kubectl (ADR-0017). The original is the
      # one every tutorial names and is effectively unmaintained; this is the
      # maintained fork, and it supports server-side apply and a state move from
      # the original if the repo ever inherits gavinbunney resources.
      #
      # 2.4, NOT 3.0. The project README says `~> 3.0`; the registry publishes
      # 3.0.0 only as beta2/beta3, so that constraint resolves to nothing —
      # verified against registry.terraform.io, not the README. 2.4.1 is the
      # highest stable release.
      source  = "alekc/kubectl"
      version = "~> 2.4"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.31"
    }
  }
}
