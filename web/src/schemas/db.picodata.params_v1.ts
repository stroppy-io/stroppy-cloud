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
}

/** root */
export interface DbPicodataParams1 {
  /** Picodata version. Picodata series (YY.MINOR); packages come from download.picodata.io. */
  version: "26.2" | "26.1" | "25.3";
  /** Tiers. Cluster tiers; one of them must be named `default` — the tier an instance lands in when it names none. */
  tiers?: Array<DbPicodataParams1Item>;
  /** Default bucket count. cluster.default_bucket_count — buckets a tier gets when it sets none of its own (Picodata default 3000). */
  default_bucket_count?: number | string;
  /** memtx memory. instance.memtx.memory per instance; Picodata's own default of 64 MB only fits a smoke test. [MB] */
  memtx_memory_mb?: number | string;
  /** SQL instruction limit. Maximum VDBE instructions per local SQL plan. Picodata defaults to 45000; full-scan workload validation can require a higher explicit limit. */
  sql_vdbe_opcode_max?: number | string;
  /** HAProxy nodes. HAProxy instances spreading pgproto clients over the instances of the default tier. */
  haproxy?: number | string;
  /** PostgreSQL protocol. Expose the PostgreSQL wire-protocol listener; stroppy drives Picodata through it. */
  pgproto?: boolean;
}
