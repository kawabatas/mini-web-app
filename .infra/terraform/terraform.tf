terraform {
  required_version = "~> 1.13.3"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.44.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.7.2"
    }
  }
}

locals {
  gcp_project_id = "mini-web-app-475006"
  region         = "asia-northeast1"
  prefix         = "mini-web-app"
}

provider "google" {
  project = local.gcp_project_id
  region  = local.region
}
