// GENERATED from schemapb schema db.external.params@1 — do not edit.
// An already-running database addressed by DSN; the stand provisions and deploys nothing.

/** root */
export interface DbExternalParams1 {
  /** Protocol. Wire protocol stroppy speaks to this database. */
  protocol: "pg" | "mysql" | "picodata" | "ydb_grpc" | "ydb_grpcs" | "cockroach";
  /** DSN. Full connection string including credentials; stored encrypted and never rendered into logs or artifacts. */
  dsn: string;
  /** Skip TLS verification. Accept the server certificate without checking the chain or hostname; needed for self-signed stands, never for a shared endpoint. */
  tls_skip_verify?: boolean;
  /** Probe query. Statement run once before the workload to prove the DSN works and the database answers. */
  probe_query?: string;
  /** Clean before run. Let the workload drop and recreate its own tables. Off by default: this database is not ours to wipe. */
  truncate_before_run?: boolean;
}
