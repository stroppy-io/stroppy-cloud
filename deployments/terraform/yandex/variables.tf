variable "networking" {
  description = "Network to create"
  type = object({
    name        = string
    external_id = string
    cidr        = string
    zone        = string
  })
  default = {
    name        = ""
    external_id = ""
    cidr        = ""
    zone        = "ru-central1-b"
  }
  validation {
    condition     = var.networking.name != ""
    error_message = "Network name should be specified"
  }
  validation {
    condition     = var.networking.external_id != ""
    error_message = "Network external ID should be specified"
  }
  validation {
    condition     = var.networking.cidr != ""
    error_message = "Network CIDR should be specified"
  }
}

variable "compute" {
  description = "Map of VMs to create"
  type = object({
    platform_id        = string
    image_id           = string
    serial_port_enable = bool
    vms = map(object({
      cores         = number
      memory        = number
      disk_size     = number
      disk_type     = string
      internal_ip   = string
      has_public_ip = bool
      user_data     = string
    }))
  })
  default = {
    platform_id        = "standard-v2"
    serial_port_enable = true
    image_id           = ""
    vms                = {}
  }
  validation {
    condition     = length(keys(var.compute.vms)) > 0
    error_message = "At least one VM should be specified"
  }
  validation {
    condition     = var.compute.image_id != ""
    error_message = "Image ID should be specified"
  }
  validation {
    condition     = var.compute.platform_id != ""
    error_message = "Platform ID should be specified"
  }
}
