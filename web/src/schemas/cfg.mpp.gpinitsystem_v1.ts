// GENERATED from schemapb schema cfg.mpp.gpinitsystem@1 — do not edit.
// gpinitsystem_config of a Greenplum-family cluster: primaries only, no standby coordinator.

/** root */
export interface CfgMppGpinitsystem1 {
  /** Array name. ARRAY_NAME, the display name of the cluster. */
  array_name?: string;
  /** Segment prefix. SEG_PREFIX of the segment data directories (gpseg0, gpseg1, …). */
  seg_prefix?: string;
  /** Primary port base. PORT_BASE: the first primary segment's port; the next ones count up. Must lie below the kernel's ephemeral port range. */
  port_base?: number | string;
  /** Encoding. ENCODING of the template database and of the cluster's databases. */
  encoding?: string;
  /** Database. DATABASE_NAME: the database gpinitsystem creates after the cluster is up. */
  database_name?: string;
  /** Coordinator keyword. Spelling of the coordinator keys: MASTER on a PostgreSQL 9.4 base, COORDINATOR on 12 and 14. Filled by the server from the engine version. */
  coordinator_keyword?: "COORDINATOR" | "MASTER";
  /** Coordinator address. COORDINATOR_HOSTNAME: the coordinator's private address. Filled by the server from topology. */
  coordinator_hostname?: string | null;
  /** Coordinator directory. COORDINATOR_DIRECTORY on the coordinator's data volume. */
  coordinator_directory?: string;
  /** Coordinator port. COORDINATOR_PORT: where clients connect. */
  coordinator_port?: number | string;
  /** Primary directories. DATA_DIRECTORY: one entry per primary segment of a segment host; entries may repeat, every segment gets its own gpseg<N> directory inside. Filled by the server from segments_per_host. */
  data_directories?: Array<string> | null;
  /** DATA_DIRECTORY. The `data_directories` list as the bash array gpinitsystem reads. */
  readonly data_directory?: string;
}
