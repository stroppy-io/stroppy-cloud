// GENERATED from schemapb schema cfg.pg_hba.conf@1 — do not edit.
// pg_hba.conf: ordered client authentication records.

/** object rule */
export interface CfgPgHbaConf1Rule {
  /** Type. Connection type the record matches. */
  type: "local" | "host" | "hostssl" | "hostnossl";
  /** Database. Database the record matches: a name, a comma-separated list, or one of all / sameuser / samerole / replication. */
  database: string;
  /** User. Role the record matches: a name, a comma-separated list, +group, or all. */
  user: string;
  /** Address. Client address for host records: CIDR (10.0.0.0/8, ::1/128), a hostname, or all / samehost / samenet. Must be empty for `local` records. */
  address?: string | null;
  /** Method. Authentication method. `oauth` exists only from PostgreSQL 18. */
  method: "trust" | "reject" | "scram-sha-256" | "md5" | "password" | "peer" | "ident" | "cert" | "gss" | "ldap" | "radius" | "pam" | "oauth";
  /** Options. Method options appended verbatim, e.g. `map=stroppy` or `clientcert=verify-full`. */
  options?: string | null;
}

/** root */
export interface CfgPgHbaConf1 {
  /** Records. Authentication records in file order: PostgreSQL uses the first record that matches. */
  rules?: Array<CfgPgHbaConf1Rule> | null;
  /** Rendered records. The `rules` list joined into pg_hba.conf lines, tab-separated, in list order. */
  readonly rules_rendered?: string;
  /** Cluster defaults. Prepend the records the server derives from the topology (local superuser peer access, replication between the cluster peers, the workload role from the load-generator subnet). Filled by the server; `no` means `rules` is the whole file. */
  include_cluster_defaults?: "yes" | "no";
  /** Rendered cluster defaults. The unconditional record every stroppy node needs: local superuser access for the agent. The remaining topology records are appended by the server as `rules`. */
  readonly cluster_defaults_rendered?: string;
}
