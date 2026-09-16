// GENERATED from schemapb schema cfg.cockroach.flags@26 — do not edit.
// CockroachDB 26.x node start flags and cluster settings.

/** root */
export interface CfgCockroachFlags26 {
  /** Store path. --store path= : the data directory, on the mounted data disk. */
  store_path?: string;
  /** Store size. --store size= : maximum store size as a percentage (25%) or a size (100GiB). Upstream default is 100%. */
  store_size?: string;
  /** Ballast size. --store ballast-size= : a reserved file that can be deleted to recover a full disk; 0 disables it. */
  ballast_size?: string;
  /** Cache. --cache : block cache, a percentage or a size. The upstream default (128MiB up to 24.x, 256MiB from 25.4) is far too small for a benchmark node. */
  cache?: string;
  /** Max SQL memory. --max-sql-memory : budget for SQL execution (sorts, joins, results). */
  max_sql_memory?: string;
  /** Max TSDB memory. --max-tsdb-memory : budget for the built-in time-series store that feeds the DB Console. */
  max_tsdb_memory?: string;
  /** Locality. --locality : ordered "key=value,key=value" pairs describing where the node runs. Filled by the server from topology (region/zone of the machine). */
  locality?: string | null;
  /** Join addresses. --join : the addresses of the other nodes. Filled by the server from topology; identical on every node. */
  join?: Array<string>;
  /** Advertise address. --advertise-addr : how other nodes and clients reach this one. Filled by the server from topology. */
  advertise_addr?: string | null;
  /** Listen address. --listen-addr : interface and port for inter-node and SQL traffic. */
  listen_addr?: string;
  /** HTTP address. --http-addr : DB Console and /_status/vars (the Prometheus endpoint the run scrapes). */
  http_addr?: string;
  /** SQL address. --sql-addr : a separate listener for client SQL; empty shares --listen-addr. */
  sql_addr?: string;
  /** Insecure. --insecure : no TLS, no authentication. Acceptable only inside the run's private network, which is where these stands live. */
  insecure?: boolean;
  /** Certificates directory. --certs-dir : node and CA certificates; read only in secure mode. */
  certs_dir?: string;
  /** Max clock offset. --max-offset : tolerated clock skew between nodes; a node that exceeds it kills itself. Must be identical on all nodes (upstream default 500ms). */
  max_offset?: string;
  /** Cluster name. --cluster-name : guards against a node joining the wrong cluster; must match on every node. */
  cluster_name?: string;
  /** Log configuration. --log : the logging configuration as a YAML string. Empty leaves the built-in default (file logs under the store). */
  log?: string;
  /** Cluster settings. SET CLUSTER SETTING <name> = <value>, applied once against the first node after the cluster is initialized. */
  cluster_settings?: Record<string, string>;
  /** WAL failover. --wal-failover : when a store's disk stalls, keep writing its WAL to another store instead of blocking. Needs at least two stores; disabled on a single-store node. */
  wal_failover?: "disabled" | "among-stores";
  /** Locality advertise address. --locality-advertise-addr : "key=value@address" pairs letting nodes in the same locality use a private address. Filled by the server from topology. */
  locality_advertise_addr?: string | null;
  /** Max disk temp storage. --max-disk-temp-storage : disk budget queries may spill to. Upstream default 32GiB; it is taken out of the store, so it must fit inside --store size. */
  max_disk_temp_storage?: string;
  /** Rendered --join. */
  readonly join_flag?: string;
  /** Rendered --locality. */
  readonly locality_flag?: string;
  /** Rendered --advertise-addr. */
  readonly advertise_flag?: string;
  /** Rendered --sql-addr. */
  readonly sql_addr_flag?: string;
  /** Rendered security flag. */
  readonly security_flag?: string;
  /** Rendered --store. */
  readonly store_flag?: string;
  /** Rendered --log. */
  readonly log_flag?: string;
  /** Rendered SQL. Derived: the SET CLUSTER SETTING block. */
  readonly settings_sql?: string;
  /** Rendered 25.x/26.x flags. Derived: --wal-failover, --max-disk-temp-storage and --locality-advertise-addr. */
  readonly modern_flags?: string;
}
