// GENERATED from schemapb schema cfg.docker.container@1 — do not edit.
// Common docker run knobs for the containers of one role.

/** root */
export interface CfgDockerContainer1 {
  /** Restart policy. --restart. Benchmarks want a crash to be visible, not silently restarted: the default is no. */
  restart_policy?: "no" | "on-failure" | "always" | "unless-stopped";
  /** Restart retries. Retry count appended to on-failure (--restart on-failure:N). Ignored for other policies. */
  restart_max_retries?: number | string;
  /** Log driver. --log-driver. json-file is what the agent tails to ship container logs. */
  log_driver?: "json-file" | "local" | "none";
  /** Log max size. --log-opt max-size: rotate a log file at this size (k/m/g suffix). */
  log_max_size?: string;
  /** Log files kept. --log-opt max-file: number of rotated files kept. */
  log_max_file?: number | string;
  /** nofile (soft). --ulimit nofile soft limit: open file descriptors per process. */
  ulimit_nofile_soft?: number | string;
  /** nofile (hard). --ulimit nofile hard limit. */
  ulimit_nofile_hard?: number | string;
  /** nproc. --ulimit nproc: max processes/threads. 0 leaves the daemon default. */
  ulimit_nproc?: number | string;
  /** memlock. --ulimit memlock in bytes; -1 = unlimited (needed when the engine locks its buffer pool). */
  ulimit_memlock?: number | string;
  /** /dev/shm size. --shm-size. PostgreSQL parallel query and pg_stat need more than the 64 MB default. [MB] */
  shm_size_mb?: number | string;
  /** PID limit. --pids-limit; -1 = unlimited. */
  pids_limit?: number | string;
  /** Limit CPU to the machine size. Emit --cpus from the machine size chosen for this role instead of letting the container use the whole host. */
  limit_cpu?: boolean;
  /** Limit memory to the machine size. Emit --memory from the machine size chosen for this role. */
  limit_memory?: boolean;
  /** CPUs. --cpus value. Filled by the server from the role's machine size when limit_cpu is on. */
  cpus?: number | null;
  /** Memory limit. --memory value. Filled by the server from the role's machine size when limit_memory is on. [MB] */
  memory_mb?: number | string | null;
  /** Privileged. --privileged. Needed only for engines that touch raw block devices (YDB pdisks). */
  privileged?: boolean;
  /** Network mode. --network. host removes the NAT hop and is what the product uses for database roles. */
  network_mode?: "host" | "bridge";
  /** Extra hosts. --add-host entries "name:ip". Filled by the server from topology so peers resolve without DNS. */
  extra_hosts?: Array<string>;
  /** Container sysctls. --sysctl key=value, applied inside the container namespace (net.* only when network_mode is bridge). */
  sysctls?: Record<string, string>;
  /** Capabilities. --cap-add. IPC_LOCK for engines that mlock memory, SYS_NICE for scheduler priority. */
  cap_add?: Array<string>;
  /** Rendered --add-host flags. */
  readonly extra_host_flags?: string;
  /** Rendered --sysctl flags. */
  readonly sysctl_flags?: string;
  /** Rendered --cap-add flags. */
  readonly cap_add_flags?: string;
  /** Rendered --restart flag. */
  readonly restart_flag?: string;
  /** Rendered --cpus/--memory flags. */
  readonly limit_flags?: string;
  /** Rendered --ulimit flags. */
  readonly ulimit_flags?: string;
  /** Rendered --privileged flag. */
  readonly privileged_flag?: string;
  /** Rendered --log-* flags. */
  readonly log_flags?: string;
}
