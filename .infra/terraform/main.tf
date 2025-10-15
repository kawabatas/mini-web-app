resource "random_id" "bucket_suffix" {
  # 4バイト（32ビット）のランダム値
  # Terraform の random_id リソースで生成される値は「バイナリデータ」を16進数（hex）で表現するため
  # 8文字のランダム英数字
  byte_length = 4
}

# GCSバケット（非公開）
resource "google_storage_bucket" "datastore_bucket" {
  name          = "${local.prefix}-bucket-${random_id.bucket_suffix.hex}"
  location      = local.region
  force_destroy = true
  storage_class = "STANDARD"

  uniform_bucket_level_access = true

  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      matches_prefix = ["backups"]
      age            = 30
    }
  }
}

# Cloud Run 用サービスアカウント
resource "google_service_account" "run_app_sa" {
  account_id   = "${local.prefix}-run-sa"
  display_name = "Cloud Run Service Account for ${local.prefix}"
}

# GCS IAM: Cloud Run に権限付与
resource "google_storage_bucket_iam_member" "run_app_admin" {
  bucket = google_storage_bucket.datastore_bucket.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.run_app_sa.email}"
}

# Cloud Run サービス
resource "google_cloud_run_v2_service" "app_service" {
  name     = local.prefix
  location = local.region

  template {
    containers {
      image = "asia-northeast1-docker.pkg.dev/${local.gcp_project_id}/${local.prefix}/main:latest"

      env {
        name  = "LOG_PROVIDER"
        value = "gcp"
      }
      env {
        name  = "LOG_LEVEL"
        value = "0"
      }
      env {
        name  = "DB_DRIVER"
        value = "sqlite"
      }
      env {
        name  = "SQLITE_SOURCE"
        value = "gcs"
      }
      env {
        name  = "STORAGE_PROVIDER"
        value = "gcs"
      }
      env {
        name  = "SQLITE_BUCKET"
        value = google_storage_bucket.datastore_bucket.name
      }
      env {
        name  = "MAINTENANCE_MODE"
        value = "off"
      }
      env {
        name  = "PERIODIC_BACKUP"
        value = "off"
      }
      env {
        name  = "PERIODIC_BACKUP_MINUTES"
        value = "10"
      }

      resources {
        limits = {
          cpu    = "1000m"
          memory = "512Mi"
        }
        startup_cpu_boost = true
      }
    }

    service_account                  = google_service_account.run_app_sa.email
    timeout                          = "2s"
    max_instance_request_concurrency = 2 // 同時リクエストを可能にしておく
    scaling {
      max_instance_count = 1 # 必ず1にする
    }
  }

  ingress = "INGRESS_TRAFFIC_ALL"

  depends_on = [google_service_account.run_app_sa]
}

data "google_iam_policy" "noauth" {
  binding {
    role = "roles/run.invoker"
    members = [
      "allUsers",
    ]
  }
}

resource "google_cloud_run_service_iam_policy" "noauth" {
  location = google_cloud_run_v2_service.app_service.location
  project  = google_cloud_run_v2_service.app_service.project
  service  = google_cloud_run_v2_service.app_service.name

  policy_data = data.google_iam_policy.noauth.policy_data
}
