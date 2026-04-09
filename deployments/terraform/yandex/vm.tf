resource "yandex_compute_instance" "vms" {
  for_each    = var.compute.vms
  name        = each.key
  platform_id = var.compute.platform_id
  network_interface {
    subnet_id  = yandex_vpc_subnet.subnet.id
    nat        = each.value.has_public_ip
    ip_address = each.value.internal_ip
  }
  resources {
    cores  = each.value.cores
    memory = each.value.memory
  }
  boot_disk {
    initialize_params {
      image_id = var.compute.image_id
      size     = each.value.disk_size
      type     = each.value.disk_type
    }
  }
  metadata = {
    user-data          = each.value.user_data
    serial-port-enable = var.compute.serial_port_enable
  }
}
