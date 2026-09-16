// GENERATED from schemapb schema db.mysql.params@1 — do not edit.
// MySQL topology and options: version, replicas, replication mode, ProxySQL.

/** root */
export interface DbMysqlParams1 {
  /** MySQL version. Server series; packages come from repo.mysql.com. */
  version: "8.4" | "8.0";
  /** Replicas. Secondary servers besides the primary; with Group Replication the whole group is primary + replicas and may not exceed 9 members. */
  replicas?: number | string;
  /** Replication mode. async = classic binlog replication; semi_sync = the semisync plugin waits for one replica ack; group = Group Replication (its own consensus, no semisync). */
  replication: "async" | "semi_sync" | "group";
  /** Single-primary group. Group Replication mode: on = one writable primary with automatic election, off = multi-primary (every member writable). */
  single_primary?: boolean;
  /** Semi-sync acks. rpl_semi_sync_source_wait_for_replica_count: how many replicas must acknowledge before the primary commits. */
  semi_sync_wait_for_slave_count?: number | string;
  /** ProxySQL nodes. ProxySQL instances routing reads and writes to the right member; 0 = clients talk to the primary directly. */
  proxysql?: number | string;
  /** Init SQL. SQL executed on the primary once the server is up, before the workload. */
  init_sql?: string;
}
