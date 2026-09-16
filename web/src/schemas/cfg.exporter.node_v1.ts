// GENERATED from schemapb schema cfg.exporter.node@1 — do not edit.
// node_exporter 1.8/1.9 flags: which collectors run and where it listens.

/** root */
export interface CfgExporterNode1 {
  /** Listen port. --web.listen-address port. 9100 is the registered node_exporter port. */
  listen_port?: number | string;
  /** Listen address. Interface part of --web.listen-address; empty means all interfaces. */
  listen_address?: string;
  /** Disable default collectors. --collector.disable-defaults: start from nothing, only `enable` runs. Off keeps the upstream default set and applies enable/disable on top. */
  disable_defaults?: boolean;
  /** Enable collectors. --collector.<name> for each. pressure (PSI) and systemd are off upstream and worth turning on for a benchmark host. */
  enable?: Array<"arp" | "conntrack" | "cpu" | "cpufreq" | "diskstats" | "edac" | "entropy" | "filefd" | "filesystem" | "hwmon" | "infiniband" | "ipvs" | "loadavg" | "mdadm" | "meminfo" | "netclass" | "netdev" | "netstat" | "nfs" | "nfsd" | "os" | "powersupplyclass" | "pressure" | "processes" | "rapl" | "schedstat" | "sockstat" | "softnet" | "stat" | "systemd" | "tapestats" | "textfile" | "thermal_zone" | "time" | "timex" | "udp_queues" | "uname" | "vmstat" | "xfs" | "zfs">;
  /** Disable collectors. --no-collector.<name> for each. Turning off the noisy default collectors keeps the scrape cheap on a loaded host. */
  disable?: Array<"arp" | "bcache" | "bonding" | "btrfs" | "conntrack" | "cpu" | "cpufreq" | "diskstats" | "dmi" | "edac" | "entropy" | "fibrechannel" | "filefd" | "filesystem" | "hwmon" | "infiniband" | "ipvs" | "loadavg" | "mdadm" | "meminfo" | "netclass" | "netdev" | "netstat" | "nfs" | "nfsd" | "nvme" | "os" | "powersupplyclass" | "pressure" | "rapl" | "schedstat" | "selinux" | "sockstat" | "softnet" | "stat" | "tapestats" | "textfile" | "thermal_zone" | "time" | "timex" | "udp_queues" | "uname" | "vmstat" | "xfs" | "zfs" | "zoneinfo">;
  /** Exclude mount points. --collector.filesystem.mount-points-exclude: regex of mount points not to report. */
  filesystem_mount_points_exclude?: string;
  /** Exclude network devices. --collector.netdev.device-exclude: regex of interfaces not to report. */
  netdev_device_exclude?: string;
  /** Exclude block devices. --collector.diskstats.device-exclude: regex of block devices not to report. */
  diskstats_device_exclude?: string;
  /** Textfile directory. --collector.textfile.directory: directory of *.prom files the agent may drop for run-scoped labels. */
  textfile_directory?: string;
  /** Log level. --log.level. */
  log_level?: "debug" | "info" | "warn" | "error";
  /** Log format. --log.format. */
  log_format?: "logfmt" | "json";
  /** Rendered --collector.* flags. */
  readonly enable_flags?: string;
  /** Rendered --no-collector.* flags. */
  readonly disable_flags?: string;
  /** Rendered --collector.disable-defaults. */
  readonly defaults_flag?: string;
  /** Rendered --collector.textfile.directory. */
  readonly textfile_flag?: string;
}
