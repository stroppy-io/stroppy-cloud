locals {
  machines_map = { for m in var.machines : m.name => m }
}

resource "yandex_compute_instance" "vms" {
  for_each    = local.machines_map
  name        = each.key
  zone        = var.zone
  platform_id = "standard-v2"

  service_account_id = var.service_account_id != "" ? var.service_account_id : null

  resources {
    cores  = each.value.cores
    memory = each.value.memory_mb / 1024
  }

  boot_disk {
    initialize_params {
      image_id = var.image_id
      size     = each.value.disk_gb
    }
  }

  network_interface {
    subnet_id = var.subnet_id
    nat       = true
  }

  metadata = {
    user-data          = each.value.cloud_init
    ssh-keys           = var.ssh_public_key != "" ? "ubuntu:${var.ssh_public_key}" : null
    serial-port-enable = true
  }

  labels = {
    role = each.value.role
  }
}
