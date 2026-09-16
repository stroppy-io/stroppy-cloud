// GENERATED from schemapb schema system.limits@1 — do not edit.
// Tenant limits: how much a tenant may run at once, how big, and for how long.

/** root */
export interface SystemLimits1 {
  /** Concurrent runs. Runs a tenant may have in flight, suite children included. */
  max_concurrent_runs: number | string;
  /** Machines per run. Upper bound on the machine count of one RunSpec. */
  max_machines_per_run: number | string;
  /** Largest size. Largest T-shirt size any role of a run may use. */
  max_size: "XS" | "S" | "M" | "L" | "XL";
  /** Longest stand. Longest a stand may be kept alive after a run finishes. */
  max_keep: string;
  /** Run retention. Ceiling for the tenant's own run_retention_days setting. [days] */
  run_retention_max_days: number | string;
}
