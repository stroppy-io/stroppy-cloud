// GENERATED from schemapb schema db.custom.params@1 — do not edit.
// Your own database image on the stand's machines, reached in a protocol stroppy speaks.

/** root */
export interface DbCustomParams1 {
  /** Image. Docker image of the database, with a tag or digest; a private registry login comes from the provider profile. */
  image: string;
  /** Protocol. Wire protocol the workload speaks to it; decides the stroppy driver and the shape of the URL. */
  protocol: "pg" | "mysql" | "picodata" | "ydb_grpc" | "ydb_grpcs" | "cockroach";
  /** Port. Port the database listens on; the container uses the host network, so it is the machine's port too. */
  port?: number | string;
  /** User. Login the workload connects with; the image has to create it, usually through env. */
  user?: string;
  /** Password. Password of that login. Empty connects without one. The stand is throwaway; this is not a secret store. */
  password?: string;
  /** Database. Database the workload opens: a name, or a path for YDB (/Root/db). */
  database?: string;
  /** Connection URL. Full URL instead of the one built from the fields above, for options they can't say. ${ip:role:db} is the first db machine. */
  url?: string;
  /** Machines. Machines running the image; the workload connects to the first, the rest are yours to join through env or command. */
  nodes?: number | string;
  /** Environment. Variables of the container: the login above, cluster settings. Values may use ${ip:...} placeholders. */
  env?: Record<string, string>;
  /** Command. Arguments replacing the image command; empty keeps the image's own. */
  cmd?: Array<string>;
  /** Data directory. Path inside the container to keep on the machine's data disk; empty keeps data in the container. */
  data_dir?: string;
  /** Ready check. Shell command run inside the container that exits 0 once the database accepts connections; the workload waits for it. Empty waits only for the container to start. */
  healthcheck?: string;
}
