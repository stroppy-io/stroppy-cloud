// GENERATED from schemapb schema db.warehousepg.params@1 — do not edit.
// WarehousePG cluster: one coordinator and a number of segment hosts. No mirrors and no standby coordinator are offered: a failed segment stops the cluster, which is what a benchmark wants to see.

/** root */
export interface DbWarehousepgParams1 {
  /** Tuning. auto = settings sized from the machine, role, topology and workload (every value the run used is in its configs); stock = the engine's own defaults. A key set in the config wins over auto. */
  tuning?: "auto" | "stock";
  /** WarehousePG version. Server release; the image is pulled per release. The PostgreSQL base of the release decides the spelling of the coordinator in gpinitsystem (MASTER_* up to 9.4, COORDINATOR_* from 12). */
  version: "7.6.1" | "6.27.7";
  /** Segment hosts. Machines that carry the primary segments; the coordinator is one more machine. */
  segment_hosts?: number | string;
  /** Segments per host. Primary segments on each segment host. 0 = auto: half of the host's vCPUs, at least 1 (what Arenadata ADB defaults to). */
  segments_per_host?: number | string;
  /** Query optimizer. Which planner the cluster uses: GPORCA (the optimizer GUC on) or the PostgreSQL-based planner (off). Set cluster-wide; every release ships with GPORCA built in. */
  optimizer?: "orca" | "postgres";
}
