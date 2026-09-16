// GENERATED from schemapb schema provider.yandex.settings@1 — do not edit.
// Yandex Cloud placement settings of a tenant provider profile (no secrets).

/** variant create of network */
export interface ProviderYandexSettings1NetworkCreate {
  kind: "create";
  /** Subnet CIDR. IPv4 range of the subnet created for the run. */
  subnet_cidr?: string;
}

/** variant existing of network */
export interface ProviderYandexSettings1NetworkExisting {
  kind: "existing";
  /** Network id. Existing VPC network the machines join. */
  network_id: string;
  /** Subnet id. Existing subnet in the chosen zone. */
  subnet_id: string;
  /** Security group id. Existing security group applied to every machine. */
  security_group_id?: string;
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
  /** Network. Create a throwaway network for every run, or place runs into an existing one. */
  network: ProviderYandexSettings1NetworkCreate | ProviderYandexSettings1NetworkExisting;
  /** Public IPs. Assign a one-to-one NAT public address to every machine. */
  public_ips?: boolean;
  /** Image family. Boot image family from the standard-images folder. */
  image_family?: "ubuntu-2404-lts" | "ubuntu-2204-lts" | "ubuntu-2004-lts" | "debian-12" | "centos-stream-9-oslogin";
  /** Preemptible. Use preemptible VMs: much cheaper, but stopped after 24h or under pressure — do not use for a measurement that must complete. */
  preemptible?: boolean;
}

