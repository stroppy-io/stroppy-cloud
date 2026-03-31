terraform {
  required_providers {
    yandex = {
      source = "yandex-cloud/yandex"
    }
  }
  required_version = ">= 0.13"
}

provider "yandex" {
  zone      = var.zone
  folder_id = var.folder_id
  # Authentication is handled via environment variables:
  #   YC_TOKEN    or  YC_SERVICE_ACCOUNT_KEY_FILE
  #   YC_CLOUD_ID (optional, folder_id is sufficient)
}
