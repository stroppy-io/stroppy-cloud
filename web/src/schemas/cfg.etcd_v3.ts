// GENERATED from schemapb schema cfg.etcd@3 — do not edit.
// etcd 3.5 configuration file for the Patroni DCS.

/** root */
export interface CfgEtcd3 {
  /** Member name. Human-readable name of this etcd member; must be unique in the cluster. Filled by the server from the node's role index. */
  name?: string;
  /** Data directory. Path to the member's data directory (WAL and snapshots). */
  data_dir?: string;
  /** Listen peer URLs. Comma-separated URLs this member listens on for peer traffic. */
  listen_peer_urls?: string;
  /** Listen client URLs. Comma-separated URLs this member listens on for client traffic. */
  listen_client_urls?: string;
  /** Advertise peer URLs. Peer URLs the other members use to reach this one. Filled by the server from the node's address. */
  initial_advertise_peer_urls?: string | null;
  /** Advertise client URLs. Client URLs Patroni uses to reach this member. Filled by the server from the node's address. */
  advertise_client_urls?: string | null;
  /** Initial cluster. Bootstrap membership as `name=peerURL` entries. Filled by the server from the etcd topology. */
  initial_cluster?: Array<string> | null;
  /** Rendered initial cluster. The initial_cluster list joined with commas — the literal value of the etcd key. */
  readonly initial_cluster_rendered?: string;
  /** Initial cluster state. `new` bootstraps a fresh cluster, `existing` joins one that already exists. */
  initial_cluster_state?: "new" | "existing";
  /** Initial cluster token. Token isolating this cluster from any other etcd cluster on the same network. */
  initial_cluster_token?: string;
  /** Heartbeat interval. Interval between leader heartbeats; roughly the round-trip time between members. [ms] */
  heartbeat_interval?: number | string;
  /** Election timeout. Time a follower waits without a heartbeat before starting an election; etcd recommends 10x the heartbeat interval. [ms] */
  election_timeout?: number | string;
  /** Snapshot count. Committed transactions between snapshots to disk. */
  snapshot_count?: number | string;
  /** Backend quota. Alarm threshold for the backend size; 0 keeps etcd's default of 2GB. [B] */
  quota_backend_bytes?: number | string;
  /** Auto compaction mode. `periodic` interprets the retention as a time window, `revision` as a number of revisions. */
  auto_compaction_mode?: "periodic" | "revision";
  /** Auto compaction retention. Retention for automatic compaction: '0' disables it, '1h' or '1' mean one hour in periodic mode. */
  auto_compaction_retention?: string;
  /** Max request bytes. Largest client request etcd accepts. [B] */
  max_request_bytes?: number | string;
  /** Enable v2 API. The deprecated etcd v2 API. Off: Patroni 3.x uses etcd3 exclusively. */
  enable_v2?: "true" | "false";
  /** Extra etcd keys. Raw top-level etcd YAML keys appended verbatim; keys must match ^[a-z_][a-z0-9_.]*$. Entry order is not preserved. */
  custom?: Record<string, string>;
  /** Rendered extra keys. The `custom` map joined into YAML lines appended at the end of the file. */
  readonly custom_rendered?: string;
}
