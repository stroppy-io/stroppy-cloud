// GENERATED from schemapb schema test.machine@1 — do not edit.

/** object boot_disk */
export interface TestMachine1BootDisk {
  /** Physical block size. YC physical block size in bytes (4096..131072 powers of two); omitted selects the smallest size that fits the disk. AWS does not expose this setting. [bytes] */
  block_size?: number | string;
  /** Size. OS disk size in GiB; must fit the selected image. [GiB] */
  gb: number | string;
  /** Type. Cloud disk type. */
  type: string;
}

/** object  */
export interface TestMachine1Item {
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

/** root */
export interface TestMachine1 {
  /** vCPU. */
  cpu?: number | string;
  /** Memory. [GB] */
  memory_gb?: number | string;
  /** Boot disk. Omit for a 40 GiB SSD OS disk. */
  boot_disk?: TestMachine1BootDisk | null;
  /** Guaranteed CPU. YC guaranteed CPU percentage; omission means 100. Unsupported on AWS. [%] */
  core_fraction?: number | string;
  /** Preemptible. Override the provider profile's preemptible/spot setting; explicit false is preserved. */
  preemptible?: boolean;
  /** Public IP. Override public addressing for this VM; outbound connectivity remains required for the agent and image pulls. */
  public_ip?: boolean;
  /** Extra disks. Secondary disks beyond the boot disk. */
  disks?: Array<TestMachine1Item>;
  /** Image. Resolved boot image (family id, image id or AMI id). */
  image?: string;
  /** Location. Zone (yandex) or availability zone (aws) the machine is created in. */
  location?: string;
  /** Instance type. Platform id (yandex) or EC2 instance type (aws) from the size table. */
  instance_type?: string;
  /** Labels. Provider labels; the run and role labels are added by the pipeline. */
  labels?: Record<string, string>;
}
