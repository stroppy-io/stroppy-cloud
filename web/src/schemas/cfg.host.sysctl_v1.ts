// GENERATED from schemapb schema cfg.host.sysctl@1 — do not edit.
// Kernel tunables (sysctl.d drop-in) applied to every machine of a run.

/** root */
export interface CfgHostSysctl1 {
  /** vm.swappiness. Swap aggressiveness, 0..200 (kernel default 60). Benchmarks keep it at 1: swapping falsifies latency. */
  vm_swappiness?: number | string;
  /** vm.overcommit_memory. 0 = heuristic (kernel default), 1 = always overcommit, 2 = strict (CommitLimit = swap + ratio% RAM). */
  vm_overcommit_memory?: 0 | 1 | 2;
  /** vm.overcommit_ratio. Percent of physical RAM added to swap to form CommitLimit; only read when vm.overcommit_memory = 2. [%] */
  vm_overcommit_ratio?: number | string;
  /** vm.dirty_ratio. Percent of available memory of dirty pages at which a writing process itself starts writeback (kernel default 20). [%] */
  vm_dirty_ratio?: number | string;
  /** vm.dirty_background_ratio. Percent of available memory of dirty pages at which the background flusher starts (kernel default 10). [%] */
  vm_dirty_background_ratio?: number | string;
  /** vm.max_map_count. Maximum number of memory-map areas per process (kernel default 65530). */
  vm_max_map_count?: number | string;
  /** vm.nr_hugepages. Number of pre-allocated persistent huge pages; 0 leaves huge pages unused (kernel default 0). */
  vm_nr_hugepages?: number | string;
  /** kernel.shmmax. Maximum size of one System V shared-memory segment, bytes. Unset leaves the kernel default (ULONG_MAX on 64-bit). [bytes] */
  kernel_shmmax?: number | string | null;
  /** kernel.shmall. Total System V shared memory the system may allocate, in PAGES (not bytes). Unset leaves the kernel default. [pages] */
  kernel_shmall?: number | string | null;
  /** kernel.sem. System V semaphore limits, four integers "SEMMSL SEMMNS SEMOPM SEMMNI". */
  kernel_sem?: string;
  /** fs.file-max. System-wide maximum number of open file handles. */
  fs_file_max?: number | string;
  /** fs.aio-max-nr. Maximum number of outstanding async I/O requests system-wide (kernel default 65536); raised for io_uring/AIO storage engines. */
  fs_aio_max_nr?: number | string;
  /** net.core.somaxconn. Maximum accept-queue length per listening socket (kernel default 4096 since 5.4). */
  net_core_somaxconn?: number | string;
  /** net.core.rmem_max. Maximum receive socket buffer a program may request with SO_RCVBUF, bytes. [bytes] */
  net_core_rmem_max?: number | string;
  /** net.core.wmem_max. Maximum send socket buffer a program may request with SO_SNDBUF, bytes. [bytes] */
  net_core_wmem_max?: number | string;
  /** net.ipv4.tcp_keepalive_time. Seconds an idle TCP connection waits before the first keepalive probe (kernel default 7200). [s] */
  net_ipv4_tcp_keepalive_time?: number | string;
  /** net.ipv4.tcp_keepalive_intvl. Seconds between keepalive probes (kernel default 75). [s] */
  net_ipv4_tcp_keepalive_intvl?: number | string;
  /** net.ipv4.tcp_keepalive_probes. Unacknowledged keepalive probes before the connection is dropped (kernel default 9). */
  net_ipv4_tcp_keepalive_probes?: number | string;
  /** net.ipv4.ip_local_port_range. Ephemeral port range for outgoing connections, "<first> <last>" (kernel default "32768 60999"). */
  net_ipv4_ip_local_port_range?: string;
  /** Transparent hugepages. /sys/kernel/mm/transparent_hugepage/enabled. Databases with their own buffer pools want never or madvise. */
  transparent_hugepage?: "never" | "madvise" | "always";
  /** Extra sysctl keys. Any further sysctl key → value, appended verbatim after the known keys. */
  custom?: Record<string, string>;
  /** Rendered custom lines. Derived: the custom map as sysctl.conf lines. */
  readonly custom_lines?: string;
}
