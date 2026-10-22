// GENERATED from schemapb schema provider.yandex.settings@1 — do not edit.
// Yandex Cloud placement settings of a tenant provider profile (no secrets).

/** variant create of network */
export interface ProviderYandexSettings1NetworkCreate {
  kind: "create";
  /** Address pool. IPv4 range the run subnets are allocated from, one block per zone sized to its machines. */
  pool_cidr?: string;
  /** Subnet CIDR. Retired: run subnets come from pool_cidr; a stored value is ignored. */
  subnet_cidr?: string;
}

/** variant existing of network */
export interface ProviderYandexSettings1NetworkExisting {
  kind: "existing";
  /** Network id. Existing VPC network the run subnets are created in. */
  network_id: string;
  /** Address pool. IPv4 range of the network kept free for runs; run subnets are allocated from it and must not overlap other subnets of the network. */
  pool_cidr: string;
  /** Route table id. Route table with a route out (e.g. 0.0.0.0/0 through a NAT gateway) for machines without public addresses. When empty, such a run creates its own NAT gateway. */
  route_table_id?: string;
}

/** root */
export interface ProviderYandexSettings1 {
  /** Cloud id. Yandex Cloud cloud id owning the folder. */
  cloud_id: string;
  /** Folder id. Folder every VM, disk and network of a run is created in. */
  folder_id: string;
  /** Zone. Availability zone of ru-central1 the run is placed in. */
  zone: "ru-central1-a" | "ru-central1-b" | "ru-central1-d" | "ru-central1-e";
  /** Distributed topology zones. Three physical zones for multi-zone topologies. When omitted, use zone and two other catalog zones. Single-zone topologies use zone. */
  zones?: Array<string>;
  /** Platform. Compute platform (CPU generation) the machines are created on. */
  platform_id?: "standard-v1" | "standard-v2" | "standard-v3" | "standard-v4a" | "amd-v1" | "highfreq-v3" | "highfreq-v4a";
  /** Network. Create one network for the profile, or place runs into an existing one. Every run gets its own subnets and security group in it. */
  network: ProviderYandexSettings1NetworkCreate | ProviderYandexSettings1NetworkExisting;
  /** Public IPs. Assign a one-to-one NAT public address to every machine. Without them a created network gets one shared NAT gateway for the profile. */
  public_ips?: boolean;
  /** Image family. Boot image family from the standard-images folder. */
  image_family?: "ubuntu-2404-lts" | "ubuntu-2204-lts" | "ubuntu-2004-lts" | "debian-12" | "centos-stream-9-oslogin";
  /** Preemptible. Use preemptible VMs: much cheaper, but stopped after 24h or under pressure — do not use for a measurement that must complete. */
  preemptible?: boolean;
}
