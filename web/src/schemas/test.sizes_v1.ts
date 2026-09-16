// GENERATED from schemapb schema test.sizes@1 — do not edit.
// Per-role machine sizing of a test: a T-shirt size and an optional disk override.

/** object boot_disk */
export interface TestSizes1MachineBootDisk {
  /** Physical block size. YC physical block size in bytes (4096..131072 powers of two); omitted selects the smallest size that fits the disk. AWS does not expose this setting. [bytes] */
  block_size?: number | string;
  /** Size. OS disk size in GiB; must fit the selected image. [GiB] */
  gb: number | string;
  /** Type. Cloud disk type. */
  type: string;
}

/** object  */
export interface TestSizes1MachineItem {
  /** Physical block size. YC physical block size in bytes (4096..131072 powers of two); omitted selects the smallest size that fits the disk. AWS does not expose this setting. [bytes] */
  block_size?: number | string;
  /** Device name. */
  name: string;
  /** Size. [GB] */
  gb: number | string;
  /** Disk type. Provider disk type id, e.g. network-ssd (yandex) or gp3 (aws). */
  type: string;
  /** Filesystem. Automatic filesystem for mounted YC disks; omit for ext4. No mount means a raw block device. */
  filesystem?: "ext4" | "xfs";
  /** Mount options. mount/fstab options, e.g. noatime; omission uses defaults. */
  mount_options?: Array<string>;
  /** Mount point. Absolute path host_prep mounts the disk at; empty leaves it raw. */
  mount?: string;
}

/** def machine */
export interface TestSizes1Machine {
  /** vCPU. */
  cpu?: number | string;
  /** Memory. [GB] */
  memory_gb?: number | string;
  /** Boot disk. Omit for a 40 GiB SSD OS disk. */
  boot_disk?: TestSizes1MachineBootDisk | null;
  /** Guaranteed CPU. YC guaranteed CPU percentage; omission means 100. Unsupported on AWS. [%] */
  core_fraction?: number | string;
  /** Preemptible. Override the provider profile's preemptible/spot setting; explicit false is preserved. */
  preemptible?: boolean;
  /** Public IP. Override public addressing for this VM; outbound connectivity remains required for the agent and image pulls. */
  public_ip?: boolean;
  /** Extra disks. Secondary disks beyond the boot disk. */
  disks?: Array<TestSizes1MachineItem>;
  /** Image. Resolved boot image (family id, image id or AMI id). */
  image?: string;
  /** Location. Zone (yandex) or availability zone (aws) the machine is created in. */
  location?: string;
  /** Instance type. Platform id (yandex) or EC2 instance type (aws) from the size table. */
  instance_type?: string;
  /** Labels. Provider labels; the run and role labels are added by the pipeline. */
  labels?: Record<string, string>;
}

/** object disk */
export interface TestSizes1RolesValueDisk {
  /** Disk type. Provider disk type id; empty keeps the size table's default. */
  type?: string;
  /** Size. Data disk size; empty keeps the size table's default. [GB] */
  gb?: number | string;
}

/** map value roles */
export interface TestSizes1RolesValue {
  /** Size. T-shirt size; the platform size table turns it into a machine. */
  size: "XS" | "S" | "M" | "L" | "XL";
  /** Machine. Override any hardware field from the selected preset; explicit disks replace the preset disk list. */
  machine?: TestSizes1Machine;
  /** Disk. Overrides the disk the size table would pick; absent means take the default. */
  disk?: TestSizes1RolesValueDisk | null;
}

/** root */
export interface TestSizes1 {
  /** Roles. Keys are topology roles (db, db-replica, proxy, runner, coordinator); the server validates them against the database topology. */
  roles: Record<string, TestSizes1RolesValue>;
}
