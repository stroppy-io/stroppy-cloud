// GENERATED from schemapb schema db.picodata.params@1 — do not edit.
// Picodata topology and options: version, tiers, sharding, memtx memory.

/** object  */
export interface DbPicodataParams1Item {
  /** Tier name. Key under cluster.tier; instances join a tier by this name. */
  name: string;
  /** Instances. How many picodata instances stroppy-cloud starts in this tier; must divide evenly into replicasets. */
  instances?: number | string;
  /** Replication factor. Instances per replicaset in this tier (Picodata default 1). */
  replication_factor?: number | string;
  /** Raft voter. Whether instances of this tier take part in the cluster raft vote (Picodata default true). */
  can_vote?: boolean;
  /** Bucket count. Sharding buckets for this tier; unset falls back to cluster.default_bucket_count. */
  bucket_count?: number | string;
  /** Replication mode. Within a replicaset: async = the leader does not wait for replicas, sync = it does (Picodata default async). */
  replication_mode?: "async" | "sync";
  /** WAL mode. 26.2 only: write = a commit waits for the WAL write, fsync = for its fsync too. Empty = fsync under tuning auto, Picodata's own write under stock; 25.3 and 26.1 always write without fsync. */
  wal_mode?: "write" | "fsync";
}

/** root */
export interface DbPicodataParams1 {
  /** Tuning. auto = settings sized from the machine, role, topology and workload (every value the run used is in its configs); stock = the engine's own defaults. A key set in the config wins over auto. */
  tuning?: "auto" | "stock";
  /** Picodata version. Picodata series (YY.MINOR); packages come from download.picodata.io. */
  version: "26.2" | "26.1" | "25.3";
  /** Tiers. Cluster tiers; one of them must be named `default` — the tier an instance lands in when it names none. */
  tiers?: Array<DbPicodataParams1Item>;
  /** Default bucket count. cluster.default_bucket_count — buckets a tier gets when it sets none of its own (Picodata default 3000). */
  default_bucket_count?: number | string;
  /** Instances per machine. Picodata instances started on one machine (one memtx transaction thread, one core each). 0 = auto: one instance per vCPU of the machine under tuning auto, one under stock; each tier instance then becomes a machine of that many instances. Otherwise machines = ceil(instances / this). Replicas of a replicaset never share a machine. Clients reach every instance only through HAProxy. */
  instances_per_machine?: number | string;
  /** memtx memory. instance.memtx.memory per instance. Empty = sized from the machine's RAM under tuning auto, 2 GB under stock; Picodata's own default of 64 MB only fits a smoke test. [MB] */
  memtx_memory_mb?: number | string;
  /** SQL instruction limit. Maximum VDBE instructions per local SQL plan; 0 = no limit. Picodata's own default is 45000, which stops full-scan queries such as workload validation. */
  sql_vdbe_opcode_max?: number | string;
  /** Concurrent SQL queries. Queries one instance runs at once (Picodata default 50; 26.1 and later). Empty = 1.5 × peak virtual users per SQL endpoint, at least 50, under tuning auto. */
  sql_runtime_concurrency_max?: number | string;
  /** Storage plan cache entries. Prepared statements in the LRU cache of a storage instance (Picodata default 50). Empty = 200 under tuning auto. */
  sql_storage_cache_count_max?: number | string;
  /** Storage plan cache size. sql_storage_cache_size_max, the LRU cache of prepared statements on a storage instance (Picodata default 5 MB). Empty = 32 MB under tuning auto. [MB] */
  sql_storage_cache_size_max_mb?: number | string;
  /** Rows per motion. Rows a virtual table of one query may collect from the storages (Picodata default 5000). Empty = 50000 under tuning auto, so large validation and batch queries do not stop at the limit. */
  sql_motion_row_max?: number | string;
  /** HAProxy nodes. HAProxy instances spreading pgproto clients over the instances of the default tier. */
  haproxy?: number | string;
  /** PostgreSQL protocol. Expose the PostgreSQL wire-protocol listener; stroppy drives Picodata through it. */
  pgproto?: boolean;
}
