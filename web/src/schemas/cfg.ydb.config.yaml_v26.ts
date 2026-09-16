// GENERATED from schemapb schema cfg.ydb.config.yaml@26 — do not edit.
// YDB 26.x static cluster configuration (config.yaml, configuration V1).

/** object  */
export interface CfgYdbConfigYaml26Item {
  /** Device path. host_configs[].drive[].path — the raw block device of one pdisk. Filled by the server from the machine's disks (cfg.host.disks raw mounts). */
  path: string;
  /** Media type. host_configs[].drive[].type — the media class YDB assigns the pdisk to. */
  type?: "ssd" | "nvme" | "rot";
}

/** object  */
export interface CfgYdbConfigYaml26Item2 {
  /** Host. hosts[].host — FQDN or address of the storage node. */
  host: string;
  /** Node id. hosts[].node_id — stable numeric id used by state_storage and the blob storage groups. */
  node_id: number | string;
  /** Host config id. hosts[].host_config_id — which host_configs entry describes this host's disks. */
  host_config_id?: number | string;
  /** Interconnect port. hosts[].port — the interconnect port (upstream default 19001). */
  port?: number | string;
  /** Data center. hosts[].location.data_center — the availability zone; mirror-3-dc spreads a group across three of them. */
  data_center?: string;
  /** Rack. hosts[].location.rack — the fault domain inside a data center. */
  rack?: string;
}

/** object  */
export interface CfgYdbConfigYaml26Item3 {
  /** Kind. domains_config.domain[].storage_pool_types[].kind — the pool name a database asks for. */
  kind?: string;
  /** Fault tolerance. pool_config.erasure_species — none is a single copy (one node), block-4-2 survives two disk failures in one AZ, mirror-3-dc survives losing a whole data center. */
  erasure_species?: "none" | "block-4-2" | "mirror-3-dc";
  /** PDisk type. pool_config.pdisk_filter[].property[].type — which media class this pool draws pdisks from. */
  pdisk_type?: "SSD" | "NVME" | "ROT";
  /** VDisk kind. pool_config.vdisk_kind. */
  vdisk_kind?: string;
  /** Box id. pool_config.box_id — the hardware box the pool lives in. */
  box_id?: number | string;
}

/** root */
export interface CfgYdbConfigYaml26 {
  /** Static group fault tolerance. static_erasure — erasure scheme of the static blob-storage group, filled by the server from topology. */
  static_erasure?: "none" | "block-4-2" | "mirror-3-dc";
  /** Drives. The pdisks of one host configuration. Filled by the server from pdisks_per_storage_node. */
  drives?: Array<CfgYdbConfigYaml26Item>;
  /** Host config id. host_configs[].host_config_id — the id every host references. */
  host_config_id?: number | string;
  /** Hosts. Every storage node of the cluster. Filled by the server from topology. */
  hosts?: Array<CfgYdbConfigYaml26Item2>;
  /** Domain. domains_config.domain[].name — the root of the cluster's scheme (/<domain>). */
  domain?: string;
  /** Storage pools. domains_config.domain[].storage_pool_types — filled by the server from the ydb params (fault_tolerance, default_disk_type). */
  storage_pool_types?: Array<CfgYdbConfigYaml26Item3>;
  /** State storage ring. domains_config.state_storage[].ring.node — the node ids holding the scheme state. Filled by the server from topology. */
  state_storage_nodes?: Array<number | string>;
  /** State storage nto_select. domains_config.state_storage[].ring.nto_select — replicas of each state-storage key; must be odd and <= the ring size. */
  state_storage_nto_select?: number | string;
  /** Require user token. security_config.enforce_user_token_requirement — reject unauthenticated requests. Benchmark stands run inside a private network and leave it off. */
  enforce_user_token_requirement?: boolean;
  /** Blob storage service set. blob_storage_config.service_set — the static group geometry (rings, fail domains, vdisk locations). Filled by the server from topology; emitted as flow-style YAML, which is valid YAML for the same document. */
  blob_storage_service_set?: unknown;
  /** Auto actor system. actor_system_config.use_auto_config — let YDB size its executor pools from cpu_count instead of hand-written executor blocks. */
  use_auto_config?: boolean;
  /** Node type. actor_system_config.node_type — STORAGE for a storage node, COMPUTE for a database node, HYBRID for a single-role cluster. */
  node_type?: "STORAGE" | "COMPUTE" | "HYBRID";
  /** CPU count. actor_system_config.cpu_count — cores the actor system may use; the server fills it from the machine size. */
  cpu_count?: number | string;
  /** gRPC port. grpc_config.port — the plaintext client endpoint (grpc://). */
  grpc_port?: number | string;
  /** Enable grpcs. Serve the TLS client endpoint as well; stroppy then connects with grpcs://. */
  grpcs?: boolean;
  /** gRPC TLS port. grpc_config.ssl_port. */
  grpc_ssl_port?: number | string;
  /** CA certificate. grpc_config.ca — PEM path of the certificate authority. */
  grpc_ca?: string;
  /** Certificate. grpc_config.cert — PEM path of the node certificate. */
  grpc_cert?: string;
  /** Private key. grpc_config.key — PEM path of the node private key. */
  grpc_key?: string;
  /** Interconnect port. interconnect_config.start_tcp port — node-to-node traffic. */
  interconnect_port?: number | string;
  /** Monitoring port. monitoring_config.monitoring_port — the embedded viewer and the /counters endpoint the run scrapes. */
  monitoring_port?: number | string;
  /** Query spilling. table_service_config.enable_query_service_spilling — spill large intermediate results to disk instead of failing the query. */
  enable_query_service_spilling?: boolean;
  /** Spilling directory. table_service_config.spilling_service_config.local_file_config.root. */
  spilling_root?: string;
  /** Spilling max size. table_service_config.spilling_service_config.local_file_config.max_total_size, in bytes. [bytes] */
  spilling_max_total_size?: number | string;
  /** Log level. log_config.default_level — 0 EMERG, 1 ALERT, 2 CRIT, 3 ERROR, 4 WARN, 5 NOTICE, 6 INFO, 7 DEBUG, 8 TRACE. */
  log_default_level?: number | string;
  /** Log to syslog. log_config.sys_log — write to syslog instead of stderr. The agent collects stderr, so this stays off. */
  log_syslog?: boolean;
  /** Log format. log_config.format. */
  log_format?: "full" | "short" | "json";
  /** Feature flags. feature_flags — a flat map of flag name to bool (enable_views, enable_column_store, ...). */
  feature_flags?: Record<string, boolean>;
  /** Extra sections. Any further top-level section: key → a flow-style YAML/JSON value, appended verbatim as `key: value` at the end of the document. */
  custom?: Record<string, string>;
  /** Memory hard limit. memory_controller_config.hard_limit_bytes — total memory the process may use. Unset lets YDB read the cgroup limit. [bytes] */
  memory_hard_limit_bytes?: number | string | null;
  /** Soft limit. memory_controller_config.soft_limit_percent — percent of the hard limit at which YDB starts shrinking caches. [%] */
  memory_soft_limit_percent?: number | string;
  /** Target utilization. memory_controller_config.target_utilization_percent — the steady-state utilization the controller aims for. [%] */
  memory_target_utilization_percent?: number | string;
  /** Shared cache min. memory_controller_config.shared_cache_min_percent — floor of the shared page cache. [%] */
  memory_shared_cache_min_percent?: number | string;
  /** Shared cache max. memory_controller_config.shared_cache_max_percent — ceiling of the shared page cache. [%] */
  memory_shared_cache_max_percent?: number | string;
  /** Rendered memory_controller_config. */
  readonly memory_controller_block?: string;
  /** Rendered host_configs[].drive. */
  readonly drive_block?: string;
  /** Rendered hosts. */
  readonly host_block?: string;
  /** Rendered storage_pool_types. */
  readonly storage_pool_block?: string;
  /** Rendered state_storage. */
  readonly state_storage_block?: string;
  /** Rendered system tablet channels. Three channels for system tablet profile 0, derived from the first configured storage pool. */
  readonly channel_profile_block?: string;
  /** Rendered feature_flags. */
  readonly feature_flag_block?: string;
  /** Rendered extra sections. */
  readonly custom_lines?: string;
  /** Rendered grpc_config TLS keys. */
  readonly grpc_tls_block?: string;
}
