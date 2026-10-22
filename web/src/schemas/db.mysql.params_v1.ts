// GENERATED from schemapb schema db.mysql.params@1 — do not edit.
// MySQL topology and options: version, replicas, replication mode, ProxySQL.

/** root */
export interface DbMysqlParams1 {
  /** Tuning. auto = settings sized from the machine, role, topology and workload (every value the run used is in its configs); stock = the engine's own defaults. A key set in the config wins over auto. */
  tuning?: "auto" | "stock";
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
  /** Buffer pool. Buffer pool size under tuning auto: auto = 70% of the database machine's RAM; fixed = buffer_pool_mb; stock = the server's 128 MB and the whole tuning as with tuning stock (the field predates tuning). An innodb_buffer_pool_size set in the config wins over every mode. */
  buffer_pool?: "auto" | "fixed" | "stock";
  /** Buffer pool size. innodb_buffer_pool_size of every database server; the run refuses a size over the machine's RAM. [MB] */
  buffer_pool_mb?: number | string;
  /** ProxySQL nodes. ProxySQL instances routing reads and writes to the right member; 0 = clients talk to the primary directly. */
  proxysql?: number | string;
  /** Init SQL. SQL executed on the primary once the server is up, before the workload. */
  init_sql?: string;
}
