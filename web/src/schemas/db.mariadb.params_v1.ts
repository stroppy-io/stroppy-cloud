// GENERATED from schemapb schema db.mariadb.params@1 — do not edit.
// MariaDB topology and options: version, replicas, replication mode (incl. Galera), proxies.

/** root */
export interface DbMariadbParams1 {
  /** MariaDB version. Server LTS series; packages come from the mariadb_repo_setup repository. */
  version: "11.8" | "11.4" | "10.11";
  /** Replicas. Secondary servers behind the primary; not used in Galera mode, where the cluster is sized by galera_nodes instead. */
  replicas?: number | string;
  /** Replication mode. async/semi_sync = one primary with standard MariaDB replication; galera = a synchronous multi-master Galera cluster with no distinguished primary. */
  replication: "async" | "semi_sync" | "galera";
  /** Galera nodes. Size of the Galera cluster; odd only, minimum 3 — quorum needs a strict majority of the last known membership. */
  galera_nodes?: 3 | 5 | 7;
  /** Buffer pool. auto = 70% of the database machine's RAM (and a disk-class innodb_io_capacity); fixed = buffer_pool_mb; stock = the server's own defaults (128 MB). An innodb_buffer_pool_size set in the config wins over every mode. */
  buffer_pool?: "auto" | "fixed" | "stock";
  /** Buffer pool size. innodb_buffer_pool_size of every database server; the run refuses a size over the machine's RAM. [MB] */
  buffer_pool_mb?: number | string;
  /** ProxySQL nodes. ProxySQL instances in front of the cluster; mutually exclusive with MaxScale. */
  proxysql?: number | string;
  /** MaxScale. Run MariaDB MaxScale 24.02 as the router instead of ProxySQL: read/write split and Galera monitoring out of the box. License: MaxScale is under the Business Source License — free in production only with fewer than three database servers; a replicated or Galera cluster behind it needs a MariaDB subscription outside benchmarking. */
  maxscale?: boolean;
  /** Init SQL. SQL executed on the primary (or the first Galera node) once the server is up, before the workload. */
  init_sql?: string;
}
