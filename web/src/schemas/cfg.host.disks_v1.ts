// GENERATED from schemapb schema cfg.host.disks@1 — do not edit.
// Data disks of a machine: filesystem, mount point and ownership, or a raw block device.

/** object  */
export interface CfgHostDisks1Item {
  /** Device. Block device path. Filled by the server from topology (the provisioned disk of this machine). */
  device: string;
  /** Raw device. No filesystem: the device is given to the engine as a raw block device (YDB pdisk). mount_point/fs/mkfs_options are ignored. */
  raw?: boolean;
  /** Mount point. Absolute directory the filesystem is mounted at. */
  mount_point?: string;
  /** Filesystem. Filesystem created on the device. xfs is the default for database data volumes. */
  fs?: "xfs" | "ext4";
  /** mkfs options. Extra flags passed to mkfs.<fs>. Empty means the distro defaults. */
  mkfs_options?: string;
  /** Mount options. Comma-separated mount(8) options. noatime/nodiratime remove a write per read. */
  mount_options?: string;
  /** Owner. chown argument for the mount point, "user:group". */
  owner?: string;
  /** Mode. Octal chmod mode for the mount point. */
  mode?: string;
}

/** root */
export interface CfgHostDisks1 {
  /** Persist in /etc/fstab. Append an /etc/fstab entry for every non-raw mount so it survives a reboot of the stand. */
  fstab?: boolean;
  /** Disks. One entry per data disk of this machine. Filled by the server from topology. */
  mounts?: Array<CfgHostDisks1Item>;
  /** Provisioning script. Derived: the shell fragment that prepares every disk. */
  readonly script?: string;
}
