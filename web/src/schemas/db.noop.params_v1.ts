// GENERATED from schemapb schema db.noop.params@1 — do not edit.
// No database — stroppy noop driver; measures the load generator ceiling.

/** root */
export interface DbNoopParams1 {
  /** Workers. Parallel noop workers inside stroppy; 0 = one per CPU of the runner. */
  workers?: number | string;
}
