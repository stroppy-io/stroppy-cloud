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
  /** ProxySQL nodes. ProxySQL instances in front of the cluster; mutually exclusive with MaxScale. */
  proxysql?: number | string;
  /** MaxScale. Run MariaDB MaxScale (25.10 line) as the router instead of ProxySQL: read/write split and Galera monitoring out of the box. */
  maxscale?: boolean;
  /** Init SQL. SQL executed on the primary (or the first Galera node) once the server is up, before the workload. */
  init_sql?: string;
}
