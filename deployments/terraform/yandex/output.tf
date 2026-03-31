output "vm_ips" {
  description = "Map of VM name to its public (NAT) IP address"
  value = {
    for name, vm in yandex_compute_instance.vms :
    name => vm.network_interface[0].nat_ip_address
  }
}
