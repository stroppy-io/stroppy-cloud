// GENERATED from schemapb schema db.cockroach.params@1 — do not edit.
// CockroachDB topology and options: version, node count, locality, TLS mode.

/** root */
export interface DbCockroachParams1 {
  /** CockroachDB version. Server series; the binary is pulled per release train (LTS lines get a year of maintenance plus a year of assistance). */
  version: "26.3" | "26.2" | "25.4" | "25.2" | "24.3" | "24.1";
  /** Nodes. Homogeneous cluster size; 1 for a single-node stand, otherwise an odd number >= 3 so Raft can hold a majority. */
  nodes?: number | string;
  /** Per-node locality. --locality for each node, in node order; leave empty for a flat cluster. When set it must have exactly one entry per node and every entry must use the same keys in the same order. */
  locality?: Array<string>;
  /** Insecure mode. Start with --insecure: no TLS and no authentication. Default on — a throwaway benchmark stand should measure the engine, not the handshake. */
  insecure?: boolean;
  /** HAProxy nodes. HAProxy instances spreading SQL clients over the nodes (cockroach gen haproxy produces an equivalent config). */
  haproxy?: number | string;
  /** Init SQL. SQL executed against the cluster once it is initialized, before the workload. */
  init_sql?: string;
}
