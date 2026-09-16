// GENERATED from schemapb schema db.pg_noop.params@1 — do not edit.
// PostgreSQL wire-protocol blackhole; measures the delivery ceiling with no storage engine behind it.

/** root */
export interface DbPgNoopParams1 {
  /** pg-noop version. Build of the pg-noop server; the binary is served through the stroppy gateway. */
  version: "0.1.2";
  /** Workers. Connection-handling workers; 0 = one per CPU of the machine. */
  workers?: number | string;
  /** Injected latency. Artificial delay added before each response, to model a storage engine that is not free. [ms] */
  latency_ms?: number | string;
  /** Error rate. Fraction of statements answered with an ErrorResponse instead of a result, to exercise the driver's error path. 0 = never, 1 = always. */
  error_rate?: number;
  /** Port. TCP port the blackhole listens on. */
  port?: number | string;
}
