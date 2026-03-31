variable "folder_id" {
  description = "Yandex Cloud folder ID"
  type        = string
  validation {
    condition     = var.folder_id != ""
    error_message = "folder_id must be specified"
  }
}

variable "zone" {
  description = "Yandex Cloud availability zone"
  type        = string
  default     = "ru-central1-b"
}

variable "subnet_id" {
  description = "Pre-existing subnet ID for VM network interfaces"
  type        = string
  validation {
    condition     = var.subnet_id != ""
    error_message = "subnet_id must be specified"
  }
}

variable "service_account_id" {
  description = "Service account ID attached to VMs (optional)"
  type        = string
  default     = ""
}

variable "ssh_public_key" {
  description = "SSH public key injected into VM metadata"
  type        = string
  default     = ""
}

variable "image_id" {
  description = "Boot disk image ID for all VMs"
  type        = string
  validation {
    condition     = var.image_id != ""
    error_message = "image_id must be specified"
  }
}

variable "machines" {
  description = "List of VMs to create"
  type = list(object({
    name       = string
    role       = string
    cores      = number
    memory_mb  = number
    disk_gb    = number
    cloud_init = string
  }))
  validation {
    condition     = length(var.machines) > 0
    error_message = "At least one machine must be specified"
  }
}
