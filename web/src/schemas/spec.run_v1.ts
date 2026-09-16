// GENERATED from schemapb schema spec.run@1 — do not edit.
// RunSpec: the resolved description of one benchmark run handed to stroppy-run.

/** object provider */
export interface SpecRun1Provider {
  /** Provider. Cloud the run is created in. */
  kind: "yandex" | "aws";
  /** Settings. Baked provider.<kind>.settings value; non-secret placement settings. */
  settings: unknown;
  /** Credentials secret. Name of the Graphene secret holding the provider credentials — never the value. */
  credentials_secret: string;
  /** ProviderConfig. Crossplane ProviderConfig the managed resources reference (t-<tenant>). */
  provider_config_name: string;
  /** Registry secret. Name of the Graphene secret with a private docker registry login, when images need one. */
  registry_secret?: string;
}

/** object  */
export interface SpecRun1NetworkItem {
  /** Port. */
  port: number | string;
  /** Protocol. */
  proto?: "tcp" | "udp";
  /** Source CIDR. Who may reach the port. */
  cidr: string;
}

/** object network */
export interface SpecRun1Network {
  /** CIDR. IPv4 range of the run network. */
  cidr?: string;
  /** Public IPs. Machines get a public address (needed when the agent has no private route out). */
  allow_public_ips?: boolean;
  /** Ingress. Security group openings beyond the intra-network traffic. */
  ingress?: Array<SpecRun1NetworkItem>;
}

/** object managed_ydb */
export interface SpecRun1ManagedYdb {
  /** Database type. Owned YC database type. */
  type: "dedicated" | "serverless";
  /** Physical zones. Dedicated database subnet zones. */
  zones?: Array<string>;
  /** Location. YC region or location identifier. */
  location_id: string;
  /** Compute preset. Dedicated node resource preset. */
  resource_preset_id?: string;
  /** Nodes. Dedicated fixed-scale node count. */
  node_count?: number | string;
  /** Storage groups. Dedicated storage group count. */
  storage_groups?: number | string;
  /** Storage type. Dedicated storage type identifier. */
  storage_type?: string;
  /** Public addresses. Whether dedicated database nodes receive public addresses. */
  assign_public_ips?: boolean;
  /** Request ceiling. Serverless request units per second ceiling; zero disables throttling. */
  throttling_rcu_limit?: number | string;
  /** Provisioned capacity. Serverless reserved request units per second. */
  provisioned_rcu_limit?: number | string;
  /** Storage ceiling. Serverless storage limit in GiB. */
  storage_size_limit_gb?: number | string;
}

/** object  */
export interface SpecRun1ItemItem {
  /** Device name. */
  name: string;
  /** Size. [GB] */
  gb: number | string;
  /** Disk type. Provider disk type id, e.g. network-ssd (yandex) or gp3 (aws). */
  type: string;
  /** Mount point. Absolute path host_prep mounts the disk at; empty leaves it raw. */
  mount?: string;
}

/** object  */
export interface SpecRun1Item {
  /** Name. */
  name: string;
  /** Role. Topology role; scrapes, containers and flows select on it. */
  role: string;
  /** vCPU. */
  cpu: number | string;
  /** Memory. [GB] */
  memory_gb: number | string;
  /** Extra disks. Secondary disks beyond the boot disk. */
  disks?: Array<SpecRun1ItemItem>;
  /** Image. Resolved boot image (family id, image id or AMI id). */
  image: string;
  /** Location. Zone (yandex) or availability zone (aws) the machine is created in. */
  location: string;
  /** Instance type. Platform id (yandex) or EC2 instance type (aws) from the size table. */
  instance_type: string;
  /** Labels. Provider labels; the run and role labels are added by the pipeline. */
  labels?: Record<string, string>;
}

/** object  */
export interface SpecRun1Item2Item {
  /** Container port. */
  container: number | string;
  /** Host port. */
  host: number | string;
}

/** object  */
export interface SpecRun1Item2Item2 {
  /** Host path. */
  source: string;
  /** Container path. */
  target: string;
  /** Read-only. */
  ro?: boolean;
}

/** object  */
export interface SpecRun1Item2Item3 {
  /** Path. Absolute path inside the container. */
  path: string;
  /** Content. Rendered config text (cfg.* schema Render output). */
  content: string;
  /** Mode. Octal file mode. */
  mode?: string;
}

/** object healthcheck */
export interface SpecRun1Item2Healthcheck {
  /** Command. */
  cmd: Array<string>;
  /** Interval. */
  interval?: string;
  /** Retries. */
  retries?: number | string;
}

/** object  */
export interface SpecRun1Item2 {
  /** Name. */
  name: string;
  /** Role. Role the container belongs to; used for logs, metrics and flows. */
  role: string;
  /** Machine. Name of the machine the container runs on. */
  machine: string;
  /** Image. Fully qualified docker image; every database is a container, no host packages. */
  image: string;
  /** Command. Argv replacing the image command. */
  cmd?: Array<string>;
  /** Entrypoint. Executable and arguments replacing the image entrypoint; omit to use the upstream image entrypoint. */
  entrypoint?: Array<string>;
  /** Environment. */
  env?: Record<string, string>;
  /** Ports. */
  ports?: Array<SpecRun1Item2Item>;
  /** Mounts. */
  mounts?: Array<SpecRun1Item2Item2>;
  /** Files. Configs rendered by the server and injected at start. */
  files?: Array<SpecRun1Item2Item3>;
  /** Scrape path. Prometheus metrics path on the container, e.g. /metrics; empty = not scraped. */
  scrape?: string;
  /** Metrics port. HTTP port for the scrape path; when omitted, uses the first container port for compatibility. */
  scrape_port?: number | string;
  /** Depends on. Container names started before this one. */
  depends_on?: Array<string>;
  /** Health check. */
  healthcheck?: SpecRun1Item2Healthcheck;
  /** Restart policy. */
  restart?: "no" | "on-failure" | "unless-stopped" | "always";
  /** Ulimits. Soft limits, e.g. nofile; -1 = unlimited. */
  ulimits?: Record<string, number | string>;
}

/** object  */
export interface SpecRun1Item3 {
  /** Role. */
  role: string;
  /** Kind. What the step does before any container starts. */
  kind: "sysctl" | "disks" | "script";
  /** Content. sysctl lines, mount spec or shell script, per kind. */
  content: string;
}

/** object  */
export interface SpecRun1Item4 {
  /** Role. */
  role: string;
  /** URL. Metrics endpoint on the machine, e.g. http://127.0.0.1:9187/metrics. */
  url: string;
  /** Job. Prometheus job label the samples land under. */
  job: string;
}

/** object  */
export interface SpecRun1Item5 {
  /** From role. */
  from_role: string;
  /** To role. Target role; set this or external, not both. */
  to_role?: string;
  /** External target. Host outside the run (registry, OTLP collector); set this or to_role. */
  external?: string;
  /** Protocol. */
  protocol: "tcp" | "http" | "grpc" | "prometheus_pull" | "otlp";
  /** Port. */
  port: number | string;
  /** Label. Human label drawn on the topology view. */
  label?: string;
}

/** object workload */
export interface SpecRun1Workload {
  /** Runner role. Role of the machine stroppy runs on. */
  runner_role: string;
  /** Stroppy image. Resolved stroppy image from the catalog for the chosen version (6.0.0+). */
  stroppy_image: string;
  /** Driver type. stroppy driverType of drivers.0. */
  driver_type: "postgres" | "mysql" | "picodata" | "ydb" | "noop";
  /** Connection URL. drivers.0.url; may carry ${ip:...} placeholders the pipeline expands after provisioning. */
  url: string;
  /** Driver options. Remaining drivers.0 keys of stroppy-config.json (bulkSize, pool, insertProgress, caCertFile, authToken…), already in stroppy's lowerCamel form. */
  driver?: unknown;
  /** Segments. Baked workload.segment@1 values in order; opaque to the pipeline beyond the fields it interprets. */
  segments: Array<unknown>;
  /** Baseline. Baked workload.stroppy@1 baseline object; absent or disabled = no machine self-check. */
  baseline?: unknown;
  /** YDB credentials reference. Graphene secret containing YC service-account credentials; a short-lived IAM token is resolved on the runner into a temporary runtime config, never a retained artifact. */
  ydb_iam_credentials_secret?: string;
  /** CA certificate. PEM the pipeline writes next to the config and points caCertFile at. */
  ca_cert?: string;
}

/** object observability */
export interface SpecRun1Observability {
  /** OTLP endpoint. Where agents forward stroppy metrics and logs. */
  otlp_endpoint?: string;
  /** OTLP headers. Comma-separated key=value headers stroppy sends with every export (otlpHeaders). */
  otlp_headers?: string;
  /** Labels. Labels stamped on every metric and log line of the run. */
  labels?: Record<string, string>;
}

/** root */
export interface SpecRun1 {
  /** Run id. Run id minted by the server; also the Graphene run id. */
  run_id: string;
  /** Tenant. Tenant slug; the Graphene namespace is t-<tenant>. */
  tenant: string;
  /** Provider. Where the run is created and with which credentials. */
  provider: SpecRun1Provider;
  /** Network. The network every machine of the run joins. */
  network: SpecRun1Network;
  /** Managed YDB. YC database created and deleted with this run; no existing database is adopted. */
  managed_ydb?: SpecRun1ManagedYdb;
  /** Machines. Every VM of the run; the pipeline creates one agent per machine. */
  machines: Array<SpecRun1Item>;
  /** Containers. Everything that runs on the machines: databases, proxies, exporters. */
  containers?: Array<SpecRun1Item2>;
  /** Host preparation. Rare pre-deploy steps run on the machine itself (machine.Command). */
  host_prep?: Array<SpecRun1Item3>;
  /** Scrapes. Agent-side Prometheus scrapes of exporters. */
  scrapes?: Array<SpecRun1Item4>;
  /** Flows. Allowed traffic between roles; drives security groups and the topology view. */
  flows?: Array<SpecRun1Item5>;
  /** Workload. What stroppy runs and where. */
  workload: SpecRun1Workload;
  /** Observability. */
  observability?: SpecRun1Observability;
  /** Keep the stand. Keep machines alive after the run for this long; 0 tears everything down at once. */
  keep?: string;
  /** Expected metrics. Metric keys the run must report; a missing key degrades the result. */
  result_expectations?: Array<string>;
}

