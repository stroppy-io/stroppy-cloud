// Package spec holds the schemas of what the pipelines receive and return.
// The server compiles a library Test into these; a pipeline never resolves
// anything itself. Secrets appear only as NAMES of Graphene secrets — a value
// never enters a spec, a Temporal history or a share snapshot.
package spec

import (
	"strings"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/provider"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/workload"
)

// namePattern is a DNS-ish identifier used for machines, containers and roles.
const namePattern = `^[a-z][a-z0-9-]{0,62}$`

// rolePattern is a topology role (db, db-replica, proxy, runner, coordinator).
const rolePattern = `^[a-z][a-z0-9_-]{0,31}$`

// secretNamePattern is a Graphene secret name.
// doc: STROPPY.MD §4 — pipeline.Secret(ctx, "yc-sa-key").
const secretNamePattern = `^[a-z0-9][a-z0-9._-]{0,62}$` //nolint:gosec // a NAME pattern, not a credential

// Run is spec.run@1 — the RunSpec: a fully resolved, self-contained
// description of one benchmark run, handed to the stroppy-run pipeline.
//
// doc: STROPPY.MD §6.1 (RunSpec sketch), §16.6; OpenAPI `Run.run_spec`
// (openapi/parts/61-components-runs.yaml, x-schema: spec.run).
//
//nolint:funlen // one flat spec: every field of the RunSpec in one place
func Run() *schemapb.Schema {
	return schemapb.NewSchema(ids.Spec("run", 1)).
		Descr("RunSpec: the resolved description of one benchmark run handed to stroppy-run.").
		Strict().Coerce().
		DefSchema("segment", workload.Segment()).
		DefSchema("driver", workload.NativeDriver()).
		DefSchema("baseline", workload.Baseline()).
		DefSchema("yandex_settings", runProviderSettings(provider.YandexSettings())).
		DefSchema("aws_settings", runProviderSettings(provider.AwsSettings())).
		Fields(
			schemapb.Str("run_id").Title("Run id").Group("Identity").
				Desc("Run id minted by the server; also the Graphene run id.").
				Format(schemapb.FormatUUID).Required(),
			schemapb.Str("tenant").Title("Tenant").Group("Identity").
				Desc("Tenant slug; the Graphene namespace is t-<tenant>.").
				Pattern(namePattern).Required(),

			schemapb.OneOf("provider", "kind").Title("Provider").Group("Provider").
				Desc("Cloud placement and named credentials; settings use the existing provider schemas.").Required().
				Variant("yandex", providerFields("yandex_settings")...).
				Variant("aws", providerFields("aws_settings")...),

			schemapb.Object("network",
				schemapb.Str("cidr").Title("CIDR").
					Desc("IPv4 range of the run network.").
					Pattern(`^(\d{1,3}\.){3}\d{1,3}/\d{1,2}$`).Default("10.130.0.0/24"),
				schemapb.Bool("allow_public_ips").Title("Public IPs").
					Desc("Machines get a public address (needed when the agent has no private route out).").
					Default(true),
				schemapb.List("ingress",
					schemapb.Object("",
						schemapb.Int64("port").Title("Port").Gte(1).Lte(65535).Required(),
						schemapb.Choice("proto").Title("Protocol").
							Opt(schemapb.StrV("tcp"), "TCP").
							Opt(schemapb.StrV("udp"), "UDP").
							Default(schemapb.StrV("tcp")),
						schemapb.Str("cidr").Title("Source CIDR").
							Desc("Who may reach the port.").
							Pattern(`^(\d{1,3}\.){3}\d{1,3}/\d{1,2}$`).Required(),
					).Strict(),
				).Title("Ingress").Desc("Security group openings beyond the intra-network traffic.").
					MaxItems(64),
			).Title("Network").Group("Network").
				Desc("The network every machine of the run joins.").Strict().Required(),

			schemapb.Object("managed_ydb",
				schemapb.Choice("type").Title("Database type").Group("Managed YDB").Desc("Owned YC database type.").Opt(schemapb.StrV("dedicated"), "Dedicated").Opt(schemapb.StrV("serverless"), "Serverless").Required(),
				schemapb.List("zones", schemapb.Str("").Pattern(`^ru-central1-[abde]$`)).Title("Physical zones").Group("Managed YDB").Desc("Dedicated database subnet zones.").MinItems(3).MaxItems(3).Unique(),
				schemapb.Str("location_id").Title("Location").Group("Managed YDB").Desc("YC region or location identifier.").Pattern(`^[a-z0-9-]{1,32}$`).Required(),
				schemapb.Str("resource_preset_id").Title("Compute preset").Group("Managed YDB").Desc("Dedicated node resource preset.").Pattern(`^[a-z0-9-]{1,64}$`),
				schemapb.Int64("node_count").Title("Nodes").Group("Managed YDB").Desc("Dedicated fixed-scale node count.").Gte(1).Lte(64),
				schemapb.Int64("storage_groups").Title("Storage groups").Group("Managed YDB").Desc("Dedicated storage group count.").Gte(1).Lte(1024),
				schemapb.Str("storage_type").Title("Storage type").Group("Managed YDB").Desc("Dedicated storage type identifier.").Pattern(`^[a-z0-9-]{1,64}$`),
				schemapb.Bool("assign_public_ips").Title("Public addresses").Group("Managed YDB").Desc("Whether dedicated database nodes receive public addresses."),
				schemapb.Int64("throttling_rcu_limit").Title("Request ceiling").Group("Managed YDB").Desc("Serverless request units per second ceiling; zero disables throttling.").Gte(0).Lte(1000000),
				schemapb.Int64("provisioned_rcu_limit").Title("Provisioned capacity").Group("Managed YDB").Desc("Serverless reserved request units per second.").Gte(0).Lte(1000000),
				schemapb.Int64("storage_size_limit_gb").Title("Storage ceiling").Group("Managed YDB").Desc("Serverless storage limit in GiB.").Gte(1).Lte(10000),
			).Title("Managed YDB").Group("Managed YDB").Desc("YC database created and deleted with this run; no existing database is adopted.").Strict(),

			schemapb.List("machines",
				schemapb.Object("",
					schemapb.Str("name").Title("Name").Pattern(namePattern).Required(),
					schemapb.Str("role").Title("Role").
						Desc("Topology role; scrapes, containers and flows select on it.").
						Pattern(rolePattern).Required(),
					schemapb.Int64("cpu").Title("vCPU").Gte(1).Lte(288).Required(),
					schemapb.Int64("memory_gb").Title("Memory").Unit("GB").Gte(1).Lte(4096).Required(),
					schemapb.Object("boot_disk",
						diskBlockSize(),
						schemapb.Int64("gb").Title("Size").Group("Storage").Desc("OS disk size in GiB; must fit the selected image.").Unit("GiB").Gte(10).Lte(262144).Required(),
						schemapb.Str("type").Title("Type").Group("Storage").Desc("Cloud disk type.").Pattern(`^[a-z0-9][a-z0-9-]{0,31}$`).Required(),
					).Title("Boot disk").Group("Storage").Desc("Omit for a 40 GiB SSD OS disk.").Strict(),
					schemapb.Int64("core_fraction").Title("Guaranteed CPU").Group("Compute").Desc("YC guaranteed CPU percentage; omission means 100. Unsupported on AWS.").Unit("%").Gte(5).Lte(100),
					schemapb.Bool("preemptible").Title("Preemptible").Group("Compute").Desc("Override the provider profile's preemptible/spot setting; explicit false is preserved."),
					schemapb.Bool("public_ip").Title("Public IP").Group("Network").Desc("Override public addressing for this VM; outbound connectivity remains required for the agent and image pulls."),
					schemapb.List("disks",
						schemapb.Object("",
							diskBlockSize(),
							schemapb.Str("name").Title("Device name").Pattern(namePattern).Required(),
							schemapb.Int64("gb").Title("Size").Unit("GB").Gte(1).Lte(262144).Required(),
							schemapb.Str("type").Title("Disk type").
								Desc("Provider disk type id, e.g. network-ssd (yandex) or gp3 (aws).").
								Pattern(`^[a-z0-9][a-z0-9-]{0,31}$`).Required(),
							schemapb.Choice("filesystem").Title("Filesystem").Group("Storage").Desc("Automatic filesystem for mounted YC disks; omit for ext4. No mount means a raw block device.").Opt(schemapb.StrV("ext4"), "ext4").Opt(schemapb.StrV("xfs"), "XFS"),
							schemapb.List("mount_options", schemapb.Str("").Pattern(`^[A-Za-z0-9_=.-]+$`).MaxLen(128)).Title("Mount options").Group("Storage").Desc("mount/fstab options, e.g. noatime; omission uses defaults.").MaxItems(32),
							schemapb.Str("mount").Title("Mount point").
								Desc("Absolute path host_prep mounts the disk at; empty leaves it raw.").
								Pattern(`^/[A-Za-z0-9._/-]*$`),
						).Strict(),
					).Title("Extra disks").Desc("Secondary disks beyond the boot disk.").MaxItems(16),
					schemapb.Str("image").Title("Image").
						Desc("Resolved boot image (family id, image id or AMI id).").
						MinLen(1).MaxLen(256).Required(),
					schemapb.Str("location").Title("Location").
						Desc("Zone (yandex) or availability zone (aws) the machine is created in.").
						MinLen(1).MaxLen(64).Required(),
					schemapb.Str("instance_type").Title("Instance type").
						Desc("Platform id (yandex) or EC2 instance type (aws) from the size table.").
						MinLen(1).MaxLen(64).Required(),
					schemapb.MapOf("labels", schemapb.Str("value").MaxLen(255)).
						Title("Labels").Desc("Provider labels; the run and role labels are added by the pipeline.").
						MaxEntries(32),
				).Strict(),
			).Title("Machines").Group("Machines").
				Desc("Every VM of the run; the pipeline creates one agent per machine.").
				MinItems(1).MaxItems(64).Required(),

			schemapb.List("containers",
				schemapb.Object("",
					schemapb.Str("name").Title("Name").Pattern(namePattern).Required(),
					schemapb.Str("role").Title("Role").
						Desc("Role the container belongs to; used for logs, metrics and flows.").
						Pattern(rolePattern).Required(),
					schemapb.Str("machine").Title("Machine").
						Desc("Name of the machine the container runs on.").
						Pattern(namePattern).Required(),
					schemapb.Choice("kind").Title("Kind").
						Desc("What the container is; the pipeline marks its record with it, so a topology view reads the kind instead of guessing from the image.").
						Opt(schemapb.StrV("database"), "Serves the workload's queries").
						Opt(schemapb.StrV("proxy"), "Stands in front of the databases").
						Opt(schemapb.StrV("coordinator"), "Keeps the cluster's consensus").
						Opt(schemapb.StrV("exporter"), "Publishes metrics about something else").
						Opt(schemapb.StrV("addon"), "One-shot helper beside the database"),
					schemapb.Str("image").Title("Image").
						Desc("Fully qualified docker image; every database is a container, no host packages.").
						MinLen(1).MaxLen(512).Required(),
					schemapb.List("cmd", schemapb.Str("").MaxLen(1024)).
						Title("Command").Desc("Argv replacing the image command.").MaxItems(64),
					schemapb.List("entrypoint", schemapb.Str("").MinLen(1).MaxLen(1024)).
						Title("Entrypoint").Group("Process").Desc("Executable and arguments replacing the image entrypoint; omit to use the upstream image entrypoint.").MaxItems(64),
					schemapb.MapOf("env", schemapb.Str("value").MaxLen(4096)).
						Title("Environment").MaxEntries(128),
					schemapb.List("ports",
						schemapb.Object("",
							schemapb.Int64("container").Title("Container port").Gte(1).Lte(65535).Required(),
							schemapb.Int64("host").Title("Host port").Desc("Host-network port; remapping is not supported.").Gte(1).Lte(65535).Required(),
						).Strict().Rule(schemapb.Rule(`this.host == this.container`, "host networking requires equal host and container ports").ID("host-network-ports")),
					).Title("Ports").MaxItems(32),
					schemapb.List("mounts",
						schemapb.Object("",
							schemapb.Str("source").Title("Host path").
								Pattern(`^/[A-Za-z0-9._/-]*$`).Required(),
							schemapb.Str("target").Title("Container path").
								Pattern(`^/[A-Za-z0-9._/-]*$`).Required(),
							schemapb.Bool("ro").Title("Read-only").Default(false),
						).Strict(),
					).Title("Mounts").MaxItems(32),
					schemapb.List("files",
						schemapb.Object("",
							schemapb.Str("path").Title("Path").
								Desc("Absolute path inside the container.").
								Pattern(`^/[A-Za-z0-9._/-]*$`).Required(),
							schemapb.Str("content").Title("Content").
								Desc("Rendered config text (cfg.* schema Render output).").
								MaxLen(1<<20).Required(),
							schemapb.Str("mode").Title("Mode").
								Desc("Octal file mode.").
								Pattern(`^0[0-7]{3}$`).Default("0644"),
						).Strict(),
					).Title("Files").Desc("Configs rendered by the server and injected at start.").
						MaxItems(32),
					schemapb.Str("scrape").Title("Scrape path").
						Desc("Prometheus metrics path on the container, e.g. /metrics; empty = not scraped.").
						Pattern(`^/[A-Za-z0-9._/-]*$`),
					schemapb.Int64("scrape_port").Title("Metrics port").Group("Observability").
						Desc("HTTP port for the scrape path; when omitted, uses the first container port for compatibility.").
						Gte(1).Lte(65535),
					schemapb.List("depends_on", schemapb.Str("").Pattern(namePattern)).
						Title("Depends on").Desc("Container names started before this one.").
						MaxItems(16).Unique(),
					schemapb.Object("healthcheck",
						schemapb.List("cmd", schemapb.Str("").MaxLen(1024)).
							Title("Command").MinItems(1).MaxItems(16).Required(),
						schemapb.Duration("interval").Title("Interval").
							Gte(time.Second).Lte(time.Hour).Default(5*time.Second),
						schemapb.Int64("retries").Title("Retries").Gte(1).Lte(100).Default(30),
					).Title("Health check").Strict(),
					schemapb.Choice("restart").Title("Restart policy").
						Opt(schemapb.StrV("no"), "Never").
						Opt(schemapb.StrV("on-failure"), "On failure").
						Opt(schemapb.StrV("unless-stopped"), "Unless stopped").
						Opt(schemapb.StrV("always"), "Always").
						Default(schemapb.StrV("unless-stopped")),
					schemapb.MapOf("ulimits", schemapb.Int64("value").Gte(-1)).
						Title("Ulimits").Desc("Soft limits, e.g. nofile; -1 = unlimited.").
						MaxEntries(16),
				).Strict(),
			).Title("Containers").Group("Containers").
				Desc("Everything that runs on the machines: databases, proxies, exporters.").
				MaxItems(256),

			schemapb.List("host_prep",
				schemapb.Object("",
					schemapb.Str("role").Title("Role").Pattern(rolePattern).Required(),
					schemapb.Str("machine").Title("Machine").Group("Machines").Desc("Optional exact machine in this role; omission selects the whole role.").Pattern(namePattern),
					schemapb.Choice("kind").Title("Kind").
						Desc("What the step does before any container starts.").
						Opt(schemapb.StrV("sysctl"), "Kernel parameters").
						Opt(schemapb.StrV("disks"), "Format and mount extra disks").
						Opt(schemapb.StrV("script"), "Shell script").
						Required(),
					schemapb.Str("content").Title("Content").
						Desc("sysctl lines, mount spec or shell script, per kind.").
						MinLen(1).MaxLen(1<<16).Required(),
				).Strict(),
			).Title("Host preparation").Group("Machines").
				Desc("Pre-deploy steps, including automatic per-disk preparation, run on the machine itself.").
				MaxItems(2048),

			schemapb.List("scrapes",
				schemapb.Object("",
					schemapb.Str("role").Title("Role").Pattern(rolePattern).Required(),
					schemapb.Str("url").Title("URL").
						Desc("Metrics endpoint on the machine, e.g. http://127.0.0.1:9187/metrics.").
						MinLen(1).MaxLen(512).Required(),
					schemapb.Str("job").Title("Job").
						Desc("Name of the container to attach this scrape to; role must match its role.").
						Pattern(rolePattern).Required(),
				).Strict(),
			).Title("Scrapes").Group("Observability").
				Desc("Agent-side Prometheus scrapes of exporters.").MaxItems(64),

			schemapb.List("flows",
				schemapb.Object("",
					schemapb.Str("from_role").Title("From role").Pattern(rolePattern).Required(),
					schemapb.Str("to_role").Title("To role").
						Desc("Target role; set this or external, not both.").
						Pattern(rolePattern),
					schemapb.Str("external").Title("External target").
						Desc("Host outside the run (registry, OTLP collector); set this or to_role.").
						MinLen(1).MaxLen(255),
					schemapb.Choice("protocol").Title("Protocol").
						Opt(schemapb.StrV("tcp"), "TCP").
						Opt(schemapb.StrV("http"), "HTTP").
						Opt(schemapb.StrV("grpc"), "gRPC").
						Opt(schemapb.StrV("prometheus_pull"), "Prometheus pull").
						Opt(schemapb.StrV("otlp"), "OTLP").
						Required(),
					schemapb.Int64("port").Title("Port").Gte(1).Lte(65535).Required(),
					schemapb.Str("label").Title("Label").
						Desc("Human label drawn on the topology view.").MaxLen(64),
				).Strict().Rule(schemapb.Rule(
					`("to_role" in this) != ("external" in this)`,
					"a flow targets exactly one of to_role or external",
				).ID("flow-target-xor")),
			).Title("Flows").Group("Network").
				Desc("Traffic relationships for the topology view; cloud security groups allow intra-network traffic and use network.ingress for extra openings.").
				MaxItems(128),

			// The compiled form of workload.stroppy@1: the server resolves the
			// version to an image, renders the connection URL (with address
			// placeholders) and the stroppy driver options; the pipeline only
			// writes them into stroppy-config.json.
			schemapb.Object("workload",
				schemapb.Str("runner_role").Title("Runner role").
					Desc("Role of the machine stroppy runs on.").
					Pattern(rolePattern).Default("runner").Required(),
				schemapb.Str("stroppy_image").Title("Stroppy image").
					Desc("Resolved stroppy image from the catalog for the chosen version (6.0.0+).").
					MinLen(1).MaxLen(512).Required(),
				// doc: stroppy `help drivers` — driverType.
				schemapb.Choice("driver_type").Title("Driver type").
					Desc("stroppy driverType of drivers.0.").
					Opt(schemapb.StrV("postgres"), "postgres").
					Opt(schemapb.StrV("mysql"), "mysql").
					Opt(schemapb.StrV("picodata"), "picodata").
					Opt(schemapb.StrV("ydb"), "ydb").
					Opt(schemapb.StrV("noop"), "noop").
					Required(),
				schemapb.Str("url").Title("Connection URL").
					Desc("drivers.0.url; may carry ${ip:...} placeholders the pipeline expands after provisioning.").
					MinLen(1).MaxLen(2048).Required(),
				schemapb.Ref("driver", "driver").Title("Driver options").
					Desc("Remaining drivers.0 keys of stroppy-config.json (bulkSize, pool, insertProgress, caCertFile, authToken…), already in stroppy's lowerCamel form."),
				schemapb.List("segments", schemapb.Ref("", "segment")).
					Title("Segments").
					Desc("Ordered workload.segment@1 values; validated with the same schema as library workloads.").
					MinItems(1).MaxItems(64).Required(),
				schemapb.Ref("baseline", "baseline").Title("Baseline").
					Desc("Baked workload.stroppy@1 baseline object; absent or disabled = no machine self-check."),
				schemapb.Str("ydb_iam_credentials_secret").Title("YDB credentials reference").Group("Workload").
					Desc("Graphene secret containing YC service-account credentials; a short-lived IAM token is resolved on the runner into a temporary runtime config, never a retained artifact.").Pattern(secretNamePattern),
				schemapb.Str("ca_cert").Title("CA certificate").
					Desc("PEM the pipeline writes next to the config and points caCertFile at.").
					MaxLen(1<<16).Secret(),
			).Title("Workload").Group("Workload").
				Desc("What stroppy runs and where.").Strict().Required(),

			schemapb.Object("observability",
				schemapb.Str("otlp_endpoint").Title("OTLP endpoint").
					Desc("Where agents forward stroppy metrics and logs.").
					MinLen(1).MaxLen(512),
				schemapb.Str("otlp_headers").Title("OTLP headers").
					Desc("Comma-separated key=value headers stroppy sends with every export (otlpHeaders).").
					MaxLen(4096).Secret(),
				schemapb.MapOf("labels", schemapb.Str("value").MaxLen(255)).
					Title("Labels").Desc("Labels stamped on every metric and log line of the run.").
					MaxEntries(32),
			).Title("Observability").Group("Observability").Strict(),

			schemapb.Duration("keep").Title("Keep the stand").Group("Lifecycle").
				Desc("Keep machines alive after the run for this long; 0 tears everything down at once.").
				Gte(0).Lte(30*24*time.Hour).Default(0),

			schemapb.List("result_expectations", schemapb.Str("").Pattern(`^[a-z][a-z0-9_]*$`).MaxLen(64)).
				Title("Expected metrics").Group("Lifecycle").
				Desc("Metric keys the run must report; a missing key degrades the result.").
				MaxItems(64).Unique(),
		).
		Rules(
			runRule(`root.workload.driver_type in ["postgres", "mysql", "noop"] || root.workload.segments.all(s, !(s.workload.script in ["tpcc/procs", "tpcb/procs"]))`, "stored-procedure workloads require PostgreSQL, MySQL or noop driver").ID("stored-procedure-driver"),
			runRule(`root.workload.driver_type != "ydb" || root.workload.segments.all(s, s.workload.script != "tpcds" || ((! ("query_stream" in s.workload) || s.workload.query_stream == null) && (!("streams" in s.workload) || s.workload.streams == 1)))`, "YDB TPC-DS supports the baked query set only").ID("tpcds-ydb-baked"),
			runRule(`root.workload.driver_type != "picodata" || root.workload.segments.all(s, s.workload.script != "tpcds" || ("no_steps" in s && "workload" in s.no_steps) || ("steps" in s && size(s.steps) > 0 && !("workload" in s.steps)))`, "Picodata TPC-DS supports loading only; exclude the workload step").ID("tpcds-picodata-load-only"),
			runRule(`!("driver" in root.workload) || !("caCertFile" in root.workload.driver) || !("ca_cert" in root.workload)`, "choose a CA file or inline CA certificate").ID("ca-source"),
			runRule(`!("scrapes" in root) || root.scrapes.all(s, "containers" in root && root.containers.exists(c, c.name == s.job && c.role == s.role))`, "each scrape must target a container by role and job=name").ID("scrape-container-exists"),
			runRule(`!("scrapes" in root) || root.scrapes.all(s, root.scrapes.filter(x, x.job == s.job && x.role == s.role).size() == 1)`, "scrape targets must be unique").ID("scrape-targets-unique"),
			runRule(`!("containers" in root) || root.containers.all(c, !("scrape" in c) || "scrape_port" in c || ("ports" in c && size(c.ports) > 0))`, "a container scrape path requires a metrics port").ID("scrape-port-required"),
			runRule(`!("containers" in root) || root.containers.all(c, !("depends_on" in c) || c.depends_on.all(d, d != c.name && d in root.containers.map(x, x.name)))`, "container dependencies must name other declared containers").ID("container-dependencies-exist"),
			runRule(`!("managed_ydb" in root) || "ydb_iam_credentials_secret" in root.workload`, "managed YDB requires named IAM credentials").ID("managed-ydb-iam"),
			runRule(`!("managed_ydb" in root) || root.managed_ydb.type != "dedicated" || ["zones", "resource_preset_id", "node_count", "storage_groups", "storage_type"].all(k, k in root.managed_ydb)`, "dedicated YDB requires placement, compute and storage fields").ID("managed-ydb-dedicated"),
			runRule(`!("managed_ydb" in root) || root.managed_ydb.type != "serverless" || "storage_size_limit_gb" in root.managed_ydb`, "serverless YDB requires a storage limit").ID("managed-ydb-serverless"),
			runRule(`!("managed_ydb" in root) || (root.provider.kind == "yandex" && root.workload.driver_type == "ydb")`, "managed YDB requires YC and the YDB driver").ID("managed-ydb-provider"),
			runRule(`!("ydb_iam_credentials_secret" in root.workload) || root.workload.driver_type == "ydb"`, "IAM credentials only apply to YDB").ID("ydb-iam-driver"),
			runRule(
				`!("machines" in root) || root.machines.all(m, root.machines.filter(x, x.name == m.name).size() == 1)`,
				"machine names must be unique",
			).ID("machine-names-unique"),
			runRule(
				`!("containers" in root) || root.containers.all(c, `+
					`root.containers.filter(x, x.name == c.name).size() == 1)`,
				"container names must be unique",
			).ID("container-names-unique"),
			runRule(
				`!("machines" in root) || !("containers" in root) || root.containers.all(c, c.machine in root.machines.map(m, m.name))`,
				"every container must land on a declared machine",
			).ID("container-machine-exists"),
			runRule(
				`!("machines" in root) || !("flows" in root) || root.flows.all(f, f.from_role in root.machines.map(m, m.role))`,
				"flow from_role must be a role of a declared machine",
			).ID("flow-from-role-exists"),
			runRule(
				`!("machines" in root) || !("flows" in root) || root.flows.all(f, `+
					`!("to_role" in f) || f.to_role in root.machines.map(m, m.role))`,
				"flow to_role must be a role of a declared machine",
			).ID("flow-to-role-exists"),
			runRule(
				`!("machines" in root) || !("scrapes" in root) || root.scrapes.all(s, s.role in root.machines.map(m, m.role))`,
				"scrape role must be a role of a declared machine",
			).ID("scrape-role-exists"),
			runRule(
				`!("machines" in root) || !("host_prep" in root) || root.host_prep.all(h, h.role in root.machines.map(m, m.role))`,
				"host_prep role must be a role of a declared machine",
			).ID("host-prep-role-exists"),
			runRule(
				`!("machines" in root) || !("workload" in root) || root.machines.filter(m, m.role == root.workload.runner_role).size() == 1`,
				"the workload requires exactly one runner machine; distributed runners are not implemented",
			).ID("runner-role-exists"),
		).
		MustBuild()
}

// Run can be validated as a root or as a suite cell.
func runRule(expr, message string) *schemapb.RuleB {
	return schemapb.Rule("[this == null ? root : this].all(r, "+strings.ReplaceAll(expr, "root", "r")+")", message)
}

// doc: https://yandex.cloud/en/docs/compute/concepts/disk#maximum-disk-size
func diskBlockSize() schemapb.FieldDef {
	return schemapb.Int64("block_size").Title("Physical block size").Group("Storage").Desc("YC physical block size in bytes (4096..131072 powers of two); omitted selects the smallest size that fits the disk. AWS does not expose this setting.").Unit("bytes").Gte(4096).Lte(131072)
}
