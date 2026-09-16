// GENERATED from schemapb schema cfg.picodata.yaml@26 — do not edit.
// Picodata 26.x instance configuration (picodata.yaml): nested iproto/http/pgproto sections.

/** object  */
export interface CfgPicodataYaml26Item {
  /** Tier name. Tier key under cluster.tier. One tier must be named "default". */
  name: string;
  /** Replication factor. cluster.tier.<name>.replication_factor — instances per replicaset in this tier. */
  replication_factor?: number | string;
  /** Can vote. cluster.tier.<name>.can_vote — whether instances of this tier take part in raft voting. */
  can_vote?: boolean;
  /** Bucket count. cluster.tier.<name>.bucket_count — sharding buckets of this tier. */
  bucket_count?: number | string;
  /** Replication mode. cluster.tier.<name>.replication_mode — 26.x only: sync makes a write wait for the replicas of the replicaset. */
  replication_mode?: "async" | "sync";
  /** WAL mode. cluster.tier.<name>.wal_mode — 26.x only: fsync makes every WAL write durable before the transaction returns. */
  wal_mode?: "write" | "fsync";
}

/** root */
export interface CfgPicodataYaml26 {
  /** Cluster name. cluster.name — every instance of one cluster must agree on it (upstream default demo). */
  cluster_name?: string;
  /** Default replication factor. cluster.default_replication_factor — replicas per replicaset for tiers that do not set their own (upstream default 1). */
  default_replication_factor?: number | string;
  /** Default bucket count. cluster.default_bucket_count — sharding buckets per tier (upstream default 3000). */
  default_bucket_count?: number | string;
  /** Shredding. cluster.shredding — overwrite deleted data files instead of unlinking them. Costs write bandwidth. */
  shredding?: boolean;
  /** Tiers. cluster.tier map. Filled by the server from the picodata params (tiers[] of the database form). */
  tiers?: Array<CfgPicodataYaml26Item>;
  /** Instance name. instance.name. Filled by the server from topology; unset lets picodata derive tier_replicaset_instance. */
  instance_name?: string | null;
  /** Replicaset name. instance.replicaset_name. Filled by the server from topology. */
  replicaset_name?: string | null;
  /** Tier. instance.tier — which tier this instance joins. Filled by the server from topology. */
  tier?: string | null;
  /** Peers. instance.peer — "host:port" of the instances used to join the cluster. Filled by the server from topology. */
  peer?: Array<string>;
  /** Failure domain. instance.failure_domain — arbitrary key/value describing where the instance runs; picodata spreads a replicaset across distinct values. Filled by the server from topology. */
  failure_domain?: Record<string, string>;
  /** Instance directory. instance.instance_dir — snapshots, xlogs and the TLS material live here; point it at the data disk. */
  instance_dir?: string;
  /** iproto listen. instance.iproto.listen — the binary protocol socket other instances connect to. */
  iproto_listen?: string;
  /** iproto advertise. instance.iproto.advertise — the address other instances should use. Filled by the server from topology. */
  iproto_advertise?: string | null;
  /** HTTP listen. instance.http.listen — the HTTP endpoint (webui, metrics). Empty disables it. */
  http_listen?: string;
  /** PostgreSQL listen. instance.pgproto.listen — the PostgreSQL wire-protocol port stroppy connects to. */
  pg_listen?: string;
  /** PostgreSQL advertise. instance.pgproto.advertise. Filled by the server from topology. */
  pg_advertise?: string | null;
  /** PostgreSQL SSL. instance.pgproto.tls.enabled — requires server.crt/server.key inside instance_dir. */
  pg_ssl?: boolean;
  /** Admin socket. instance.admin_socket — the unix socket `picodata admin` attaches to. */
  admin_socket?: string;
  /** Boot timeout. instance.boot_timeout — seconds an instance waits to join the cluster before giving up (upstream default 7200). [s] */
  boot_timeout?: number | string;
  /** Audit destination. instance.audit — "file:<path>", "pipe:<command>" or "syslog:". Empty disables the audit log. */
  audit?: string;
  /** memtx memory. instance.memtx.memory — the in-memory storage arena; minimum 32M. The product default is 2G (upstream 64M is far too small for a benchmark). */
  memtx_memory?: string;
  /** memtx max tuple size. instance.memtx.max_tuple_size — largest single tuple (upstream default 1M). */
  memtx_max_tuple_size?: string;
  /** vinyl memory. instance.vinyl.memory — write buffer of the on-disk engine (upstream default 128M). */
  vinyl_memory?: string;
  /** vinyl cache. instance.vinyl.cache — read cache of the on-disk engine (upstream default 128M). */
  vinyl_cache?: string;
  /** vinyl read threads. instance.vinyl.read_threads (upstream default 1). */
  vinyl_read_threads?: number | string;
  /** vinyl write threads. instance.vinyl.write_threads (upstream default 4). */
  vinyl_write_threads?: number | string;
  /** Log level. instance.log.level. */
  log_level?: "fatal" | "system" | "error" | "crit" | "warn" | "info" | "verbose" | "debug";
  /** Log format. instance.log.format. */
  log_format?: "plain" | "json";
  /** Log destination. instance.log.destination — "file:<path>", "pipe:<command>" or "syslog:". Empty logs to stderr, which is what the agent collects. */
  log_destination?: string;
  /** Extra instance keys. Further instance.* keys, emitted verbatim at the end of the instance section as `key: value`. */
  custom?: Record<string, string>;
  /** memtx system memory. instance.memtx.system_memory — 26.x only: the arena picodata reserves for its own system spaces (upstream default 256M). */
  memtx_system_memory?: string;
  /** WAL directory. instance.wal_dir — 26.x only: where xlogs are written; separate it from instance_dir to put the WAL on its own disk. */
  wal_dir?: string;
  /** Backup directory. instance.backup_dir — 26.x only: destination of `picodata backup`. */
  backup_dir?: string;
  /** Rendered cluster.tier. */
  readonly tier_block?: string;
  /** Rendered instance.peer. */
  readonly peer_line?: string;
  /** Rendered instance.failure_domain. */
  readonly failure_domain_line?: string;
  /** Rendered extra instance keys. */
  readonly custom_lines?: string;
  /** Rendered instance identity. */
  readonly identity_lines?: string;
  /** Rendered the pg advertise address. */
  readonly pg_advertise_line?: string;
  /** Rendered instance.iproto.advertise. */
  readonly iproto_advertise_line?: string;
}
