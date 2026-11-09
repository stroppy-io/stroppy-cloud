// GENERATED from schemapb schema cfg.mpp.gucs@1 — do not edit.
// GUCs applied with gpconfig to a Greenplum-family cluster, one list per role.

/** root */
export interface CfgMppGucs1 {
  /** Segment memory limit. Memory one segment instance (or the coordinator) may use for all its queries; the server cancels the biggest query above it. Engine default 8192, which caps a big host unless raised. [MB] */
  gp_vmem_protect_limit?: number | string | null;
  /** Max statement memory. Upper bound of statement_mem for any session. Engine default 2000; statement_mem must stay below it, so it is written first. [MB] */
  max_statement_mem?: number | string | null;
  /** Statement memory. Memory one query may use on one segment before operators spill to disk. Engine default 125; must be below max_statement_mem. [MB] */
  statement_mem?: number | string | null;
  /** Work memory. Memory of one sort or hash step before it spills (engine default 32). [MB] */
  work_mem?: number | string | null;
  /** Shared buffers. Shared buffer pool of each segment instance (engine default 125). Analytic scans barely use it. [MB] */
  shared_buffers?: number | string | null;
  /** Effective cache size. The PostgreSQL planner's guess of the cache one segment instance can count on; GPORCA does not read it. [MB] */
  effective_cache_size?: number | string | null;
  /** Runaway detector threshold. Share of gp_vmem_protect_limit above which the server starts cancelling the biggest queries (engine default 90). [%] */
  runaway_detector_activation_percent?: number | string | null;
  /** Resource manager. How concurrency and memory of queries are managed: queue = resource queues (all engines), group = cgroups (needs a cgroup setup the images do not have), none = no manager (7.x and Cloudberry). */
  gp_resource_manager?: "queue" | "group" | "none" | null;
  /** GPORCA. on = GPORCA plans queries, off = the PostgreSQL-based planner. The optimizer parameter of the database sets it. */
  optimizer?: "on" | "off" | null;
  /** Automatic statistics. When a statement triggers ANALYZE on its own: none, on_change or on_no_stats. 6.x engines default to on_no_stats, 7.x and Cloudberry to none. */
  gp_autostats_mode?: "none" | "on_change" | "on_no_stats" | null;
  /** Autovacuum. Background vacuum and analyze of the catalogs. */
  autovacuum?: "on" | "off" | null;
  /** Interconnect type. Transport of motion data between segments: udpifc (every engine's default), tcp or proxy. */
  gp_interconnect_type?: "udpifc" | "tcp" | "proxy" | null;
  /** Interconnect packet size. Largest UDP packet of the interconnect (engine default 8192, which assumes jumbo frames). 1420 fits a 1500-byte MTU without IP fragmentation. [bytes] */
  gp_max_packet_size?: number | string | null;
  /** Receive queue depth. Packets a receiver queues per sender (engine default 4). [packets] */
  gp_interconnect_queue_depth?: number | string | null;
  /** Send queue depth. Packets a sender keeps in flight per receiver (engine default 2). [packets] */
  gp_interconnect_snd_queue_depth?: number | string | null;
  /** Connections. Client connections of the coordinator; on segments every session of the coordinator opens one connection per slice, so segments need at least 3 times the coordinator's value. */
  max_connections?: number | string | null;
  /** FTS probe timeout. How long the fault prober waits for a segment to answer (engine default 20). [s] */
  gp_fts_probe_timeout?: number | string | null;
  /** FTS probe interval. Time between probes of all segments (engine default 60). [s] */
  gp_fts_probe_interval?: number | string | null;
  /** Segment connect timeout. How long the coordinator waits for a segment to accept a connection (engine default 180). [s] */
  gp_segment_connect_timeout?: number | string | null;
  /** Max WAL size. WAL size that triggers a checkpoint. PostgreSQL 12 and 14 bases only (7.x, 8.x and Cloudberry). [MB] */
  max_wal_size?: number | string | null;
  /** Extra settings. Raw `key = value` settings appended verbatim at the end of the file; keys must match ^[a-z_][a-z0-9_.]*$. Entry order is not preserved. */
  custom?: Record<string, string>;
  /** Rendered extra settings. The `custom` map joined into config lines; the template prints it at the end of the file. */
  readonly custom_rendered?: string;
}
