// GENERATED from schemapb schema system.stroppy_catalog@1 — do not edit.
// Platform catalog of stroppy versions, their images, protocols and workload scripts.

/** object  */
export interface SystemStroppyCatalog1ItemItemItem {
  /** Step id. */
  id: string;
  /** Title. */
  title?: string;
  /** Phase. Where the step sits in a run: preparing, measuring, cleaning up. */
  phase?: "bootstrap" | "workload" | "teardown";
}

/** object  */
export interface SystemStroppyCatalog1ItemItemItem2 {
  /** Flag name. Typed parameter flag without dashes (load-workers). */
  name: string;
  /** Config key. Key of the stroppy-config.json params object (loadWorkers). */
  config: string;
  /** Scope. */
  scope?: "run" | "workload";
  /** Type. */
  type: "string" | "bool" | "int" | "int64" | "float64" | "duration";
  /** Description. */
  description?: string;
  /** Default. Declared default as stroppy reports it; null when contextual. */
  default?: unknown | null;
  /** Default rule. */
  default_description?: string;
  /** Env name. Legacy environment variable of the parameter. */
  env?: string;
}

/** object  */
export interface SystemStroppyCatalog1ItemItem {
  /** Script id. Workload script id as passed to `stroppy run`. */
  id: string;
  /** Title. */
  title: string;
  /** Description. */
  description?: string;
  /** Protocols. Protocols this script runs on; procs variants are pg/mysql only. */
  protocols?: Array<"pg" | "mysql" | "picodata" | "ydb_grpc" | "ydb_grpcs" | "cockroach" | "noop">;
  /** Steps. Steps the script declares; the segment form filters on them. */
  steps: Array<SystemStroppyCatalog1ItemItemItem>;
  /** Parameters. Typed parameters the script declares, as probed from the build; the segment form is generated from the known ones and passes the rest through extra_params. */
  params?: Array<SystemStroppyCatalog1ItemItemItem2>;
}

/** object  */
export interface SystemStroppyCatalog1Item {
  /** Version. Stroppy build: a release (6.0.0) or a nightly of one commit (nightly-<sha>). 6.0.0 is the minimum. */
  version: string;
  /** Image. Docker image of that build, e.g. ghcr.io/stroppy-io/stroppy:v6.0.0.62. */
  image: string;
  /** Has baseline. The build ships `stroppy baseline` (6.0.0+); lets the workload form offer the runner self-check. */
  baseline?: boolean;
  /** Default. The version a new workload starts with; exactly one version carries it. */
  default?: boolean;
  /** Deprecated. Still runnable, hidden from the picker for new workloads. */
  deprecated?: boolean;
  /** Protocols. Database protocols this build has drivers for. */
  protocols: Array<"pg" | "mysql" | "picodata" | "ydb_grpc" | "ydb_grpcs" | "cockroach" | "noop">;
  /** Scripts. Workload scripts this build ships. */
  scripts: Array<SystemStroppyCatalog1ItemItem>;
}

/** root */
export interface SystemStroppyCatalog1 {
  /** Source. Where the catalog came from: hand-written, read from a stroppy release, or probed. */
  source?: "static" | "release" | "probe";
  /** Versions. Every stroppy build the platform offers. */
  versions: Array<SystemStroppyCatalog1Item>;
}
