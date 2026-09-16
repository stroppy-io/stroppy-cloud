package cfg

import (
	"fmt"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Ydb25 is cfg.ydb.config.yaml@25 (YDB 25.x) and Ydb26 is
// cfg.ydb.config.yaml@26 (YDB 26.x): the static cluster configuration every
// storage and database node is started with.
//
// doc: https://ydb.tech/docs/en/reference/configuration/
//   - host_configs / hosts / domains_config / blob_storage_config /
//     actor_system_config / log_config / table_service_config /
//     security_config / feature_flags all have their own page under that path
//   - memory_controller_config:
//     https://ydb.tech/docs/en/reference/configuration/memory_controller_config
//   - the per-section index is versionable:
//     https://ydb.tech/docs/en/deploy/configuration/config?version=v26.2
//
// Why 25 and 26 render the same document. YDB 25.1 introduced "configuration
// V2" — the same sections wrapped in a `metadata:` (kind: MainConfig, cluster,
// version) plus `config:` envelope and applied with
// `ydb admin cluster config replace`
// (https://ydb.tech/docs/en/devops/configuration-management/configuration-v2/config-overview).
// It is still marked experimental and V1 is what the docs recommend for
// production
// (https://ydb.tech/docs/en/devops/configuration-management/compare-configs),
// and 26.x neither requires V2 nor removes V1: the v26.2 section index lists
// the identical top-level sections. So both majors render the flat V1
// document; if a future major makes V2 mandatory, that major gets the
// envelope and these two stay as they are.
//
// Note on `hosts`: the reference spells the topology block `location`
// (data_center / rack / unit); the older `walle_location` form is not used.
//
// @24 was dropped together with YDB 24.x leaving db.ydb.params.

// Ydb25 is cfg.ydb.config.yaml@25.
func Ydb25() *schemapb.Schema { return ydbConfig(25) }

// Ydb26 is cfg.ydb.config.yaml@26.
func Ydb26() *schemapb.Schema { return ydbConfig(26) }

//nolint:funlen // one flat static-configuration surface
func ydbConfig(major uint64) *schemapb.Schema {
	fields := []schemapb.FieldDef{
		// doc: https://ydb.tech/docs/en/devops/deployment-options/manual/initial-deployment/deployment-configuration-v1
		schemapb.Choice("static_erasure").Title("Static group fault tolerance").Group("Cluster").
			Desc("static_erasure — erasure scheme of the static blob-storage group, filled by the server from topology.").
			Opt(schemapb.StrV("none"), "none — no redundancy").
			Opt(schemapb.StrV("block-4-2"), "block-4-2").
			Opt(schemapb.StrV("mirror-3-dc"), "mirror-3-dc").
			Default(schemapb.StrV("none")),
		// --- host_configs --------------------------------------------------
		// doc: host_configs[].drive[].type — ssd | nvme | rot
		schemapb.List("drives",
			schemapb.Object("",
				schemapb.Str("path").Title("Device path").Group("Cluster").
					Desc("host_configs[].drive[].path — the raw block device of one pdisk. Filled by the server from the machine's disks (cfg.host.disks raw mounts).").
					MinLen(1).MaxLen(256).Required().
					Examples(schemapb.StrV("/dev/disk/by-partlabel/ydb_disk_ssd_01")),
				schemapb.Choice("type").Title("Media type").Group("Storage").
					Desc("host_configs[].drive[].type — the media class YDB assigns the pdisk to.").
					Opt(schemapb.StrV("ssd"), "SSD").
					Opt(schemapb.StrV("nvme"), "NVMe").
					Opt(schemapb.StrV("rot"), "ROT — rotational disk").
					Default(schemapb.StrV("ssd")),
			).Strict(),
		).Title("Drives").Group("Cluster").
			Desc("The pdisks of one host configuration. Filled by the server from pdisks_per_storage_node.").
			MaxItems(32),
		schemapb.Int64("host_config_id").Title("Host config id").Group("Storage").
			Desc("host_configs[].host_config_id — the id every host references.").
			Gte(1).Lte(1000).Default(1),

		// --- hosts (cluster-filled) ------------------------------------------
		schemapb.List("hosts",
			schemapb.Object("",
				schemapb.Str("host").Title("Host").Group("Cluster").
					Desc("hosts[].host — FQDN or address of the storage node.").
					MinLen(1).MaxLen(256).Required(),
				schemapb.Int64("node_id").Title("Node id").Group("Cluster").
					Desc("hosts[].node_id — stable numeric id used by state_storage and the blob storage groups.").
					Gte(1).Lte(10000).Required(),
				schemapb.Int64("host_config_id").Title("Host config id").Group("Cluster").
					Desc("hosts[].host_config_id — which host_configs entry describes this host's disks.").
					Gte(1).Lte(1000).Default(1),
				schemapb.Int64("port").Title("Interconnect port").Group("Cluster").
					Desc("hosts[].port — the interconnect port (upstream default 19001).").
					Gte(1).Lte(65535).Default(19001),
				schemapb.Str("data_center").Title("Data center").Group("Cluster").
					Desc("hosts[].location.data_center — the availability zone; mirror-3-dc spreads a group across three of them.").
					MaxLen(64).Default("1"),
				schemapb.Str("rack").Title("Rack").Group("Cluster").
					Desc("hosts[].location.rack — the fault domain inside a data center.").
					MaxLen(64).Default("1"),
			).Strict(),
		).Title("Hosts").Group("Cluster").
			Desc("Every storage node of the cluster. Filled by the server from topology.").
			MaxItems(256),

		// --- domains_config ---------------------------------------------------
		schemapb.Str("domain").Title("Domain").Group("Domain").
			Desc("domains_config.domain[].name — the root of the cluster's scheme (/<domain>).").
			Pattern(`^[A-Za-z][A-Za-z0-9_-]{0,31}$`).Default("Root"),
		schemapb.List("storage_pool_types",
			schemapb.Object("",
				schemapb.Str("kind").Title("Kind").Group("Domain").
					Desc("domains_config.domain[].storage_pool_types[].kind — the pool name a database asks for.").
					Pattern(`^[a-z][a-z0-9_-]{0,31}$`).Default("ssd"),
				schemapb.Choice("erasure_species").Title("Fault tolerance").Group("Domain").
					Desc("pool_config.erasure_species — none is a single copy (one node), block-4-2 survives two disk failures in one AZ, mirror-3-dc survives losing a whole data center.").
					Opt(schemapb.StrV("none"), "none — no redundancy").
					Opt(schemapb.StrV("block-4-2"), "block-4-2").
					Opt(schemapb.StrV("mirror-3-dc"), "mirror-3-dc").
					Default(schemapb.StrV("none")),
				schemapb.Choice("pdisk_type").Title("PDisk type").Group("Domain").
					Desc("pool_config.pdisk_filter[].property[].type — which media class this pool draws pdisks from.").
					Opt(schemapb.StrV("SSD"), "SSD").
					Opt(schemapb.StrV("NVME"), "NVMe").
					Opt(schemapb.StrV("ROT"), "ROT").
					Default(schemapb.StrV("SSD")),
				schemapb.Str("vdisk_kind").Title("VDisk kind").Group("Domain").
					Desc("pool_config.vdisk_kind.").
					Pattern(`^[A-Za-z0-9_]{1,32}$`).Default("Default"),
				schemapb.Int64("box_id").Title("Box id").Group("Domain").
					Desc("pool_config.box_id — the hardware box the pool lives in.").
					Gte(1).Lte(1000).Default(1),
			).Strict(),
		).Title("Storage pools").Group("Domain").
			Desc("domains_config.domain[].storage_pool_types — filled by the server from the ydb params (fault_tolerance, default_disk_type).").
			MaxItems(8),
		schemapb.List("state_storage_nodes", schemapb.Int64("").Gte(1).Lte(10000)).
			Title("State storage ring").Group("Cluster").
			Desc("domains_config.state_storage[].ring.node — the node ids holding the scheme state. Filled by the server from topology.").
			MaxItems(64).Unique(),
		schemapb.Int64("state_storage_nto_select").Title("State storage nto_select").Group("Cluster").
			Desc("domains_config.state_storage[].ring.nto_select — replicas of each state-storage key; must be odd and <= the ring size.").
			Gte(1).Lte(9).Default(1),
		// doc: security_config
		schemapb.Bool("enforce_user_token_requirement").Title("Require user token").Group("Security").
			Desc("security_config.enforce_user_token_requirement — reject unauthenticated requests. Benchmark stands run inside a private network and leave it off.").
			Default(false),

		// --- blob_storage_config ---------------------------------------------
		schemapb.JSON("blob_storage_service_set").Title("Blob storage service set").Group("Cluster").
			Desc("blob_storage_config.service_set — the static group geometry (rings, fail domains, vdisk locations). Filled by the server from topology; emitted as flow-style YAML, which is valid YAML for the same document."),

		// --- actor_system_config ----------------------------------------------
		schemapb.Bool("use_auto_config").Title("Auto actor system").Group("Actor system").
			Desc("actor_system_config.use_auto_config — let YDB size its executor pools from cpu_count instead of hand-written executor blocks.").
			Default(true),
		schemapb.Choice("node_type").Title("Node type").Group("Actor system").
			Desc("actor_system_config.node_type — STORAGE for a storage node, COMPUTE for a database node, HYBRID for a single-role cluster.").
			Opt(schemapb.StrV("STORAGE"), "STORAGE").
			Opt(schemapb.StrV("COMPUTE"), "COMPUTE").
			Opt(schemapb.StrV("HYBRID"), "HYBRID").
			Default(schemapb.StrV("HYBRID")),
		schemapb.Int64("cpu_count").Title("CPU count").Group("Actor system").
			Desc("actor_system_config.cpu_count — cores the actor system may use; the server fills it from the machine size.").
			Gte(1).Lte(1024).Default(4),

		// --- grpc / interconnect / monitoring ---------------------------------
		// TODO verify: grpc_config / interconnect_config / monitoring_config
		// have no dedicated page on the current docs index; the key spellings
		// below follow ydb/deploy/yaml_config_examples in the YDB repository.
		schemapb.Int64("grpc_port").Title("gRPC port").Group("Network").
			Desc("grpc_config.port — the plaintext client endpoint (grpc://).").
			Gte(1).Lte(65535).Default(2136),
		schemapb.Bool("grpcs").Title("Enable grpcs").Group("Network").
			Desc("Serve the TLS client endpoint as well; stroppy then connects with grpcs://.").
			Default(false),
		schemapb.Int64("grpc_ssl_port").Title("gRPC TLS port").Group("Network").
			Desc("grpc_config.ssl_port.").
			When("root.grpcs").Gte(1).Lte(65535).Default(2135),
		schemapb.Str("grpc_ca").Title("CA certificate").Group("Network").
			Desc("grpc_config.ca — PEM path of the certificate authority.").
			When("root.grpcs").Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/etc/ydb/certs/ca.pem"),
		schemapb.Str("grpc_cert").Title("Certificate").Group("Network").
			Desc("grpc_config.cert — PEM path of the node certificate.").
			When("root.grpcs").Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/etc/ydb/certs/node.crt"),
		schemapb.Str("grpc_key").Title("Private key").Group("Network").
			Desc("grpc_config.key — PEM path of the node private key.").
			When("root.grpcs").Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/etc/ydb/certs/node.key"),
		schemapb.Int64("interconnect_port").Title("Interconnect port").Group("Network").
			Desc("interconnect_config.start_tcp port — node-to-node traffic.").
			Gte(1).Lte(65535).Default(19001),
		schemapb.Int64("monitoring_port").Title("Monitoring port").Group("Network").
			Desc("monitoring_config.monitoring_port — the embedded viewer and the /counters endpoint the run scrapes.").
			Gte(1).Lte(65535).Default(8765),

		// --- table_service_config ---------------------------------------------
		schemapb.Bool("enable_query_service_spilling").Title("Query spilling").Group("Table service").
			Desc("table_service_config.enable_query_service_spilling — spill large intermediate results to disk instead of failing the query.").
			Default(true),
		schemapb.Str("spilling_root").Title("Spilling directory").Group("Table service").
			Desc("table_service_config.spilling_service_config.local_file_config.root.").
			Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/var/lib/ydb/spilling"),
		schemapb.Int64("spilling_max_total_size").Title("Spilling max size").Group("Table service").
			Desc("table_service_config.spilling_service_config.local_file_config.max_total_size, in bytes.").
			Unit("bytes").Gte(67108864).Lte(4398046511104).Default(21474836480),

		// --- log_config ---------------------------------------------------------
		// doc: log_config.default_level is a NUMBER: 0 EMERG .. 8 TRACE,
		// upstream default 5 (NOTICE).
		schemapb.Int64("log_default_level").Title("Log level").Group("Logging").
			Desc("log_config.default_level — 0 EMERG, 1 ALERT, 2 CRIT, 3 ERROR, 4 WARN, 5 NOTICE, 6 INFO, 7 DEBUG, 8 TRACE.").
			Gte(0).Lte(8).Default(5),
		schemapb.Bool("log_syslog").Title("Log to syslog").Group("Logging").
			Desc("log_config.sys_log — write to syslog instead of stderr. The agent collects stderr, so this stays off.").
			Default(false),
		schemapb.Choice("log_format").Title("Log format").Group("Logging").
			Desc("log_config.format.").
			Opt(schemapb.StrV("full"), "full").
			Opt(schemapb.StrV("short"), "short").
			Opt(schemapb.StrV("json"), "json").
			Default(schemapb.StrV("full")),

		// --- feature_flags -------------------------------------------------------
		schemapb.MapOf("feature_flags", schemapb.Bool("value")).
			Title("Feature flags").Group("Feature flags").
			Desc("feature_flags — a flat map of flag name to bool (enable_views, enable_column_store, ...).").
			MaxEntries(64).
			Rules(schemapb.Rule(
				`this.all(k, k.matches("^[a-z][a-z0-9_]*$"))`,
				"feature flag names are lower_snake identifiers").ID("flag-name-shape")),

		// --- escape hatch --------------------------------------------------------
		schemapb.MapOf("custom", schemapb.Str("value").MaxLen(4096)).
			Title("Extra sections").Group("Custom").
			Desc("Any further top-level section: key → a flow-style YAML/JSON value, appended verbatim as `key: value` at the end of the document.").
			MaxEntries(32).
			Rules(schemapb.Rule(
				`this.all(k, k.matches("^[a-z][a-z0-9_]*$"))`,
				"section names are lower_snake identifiers").ID("custom-key-shape")),
	}

	// doc: reference/configuration/memory_controller_config
	fields = append(fields,
		schemapb.Int64("memory_hard_limit_bytes").Title("Memory hard limit").Group("Memory controller").
			Desc("memory_controller_config.hard_limit_bytes — total memory the process may use. Unset lets YDB read the cgroup limit.").
			Unit("bytes").Gte(1073741824).Nullable(),
		schemapb.Int64("memory_soft_limit_percent").Title("Soft limit").Group("Memory controller").
			Desc("memory_controller_config.soft_limit_percent — percent of the hard limit at which YDB starts shrinking caches.").
			Unit("%").Gte(1).Lte(100).Default(75),
		schemapb.Int64("memory_target_utilization_percent").Title("Target utilization").Group("Memory controller").
			Desc("memory_controller_config.target_utilization_percent — the steady-state utilization the controller aims for.").
			Unit("%").Gte(1).Lte(100).Default(50),
		schemapb.Int64("memory_shared_cache_min_percent").Title("Shared cache min").Group("Memory controller").
			Desc("memory_controller_config.shared_cache_min_percent — floor of the shared page cache.").
			Unit("%").Gte(0).Lte(100).Default(20),
		schemapb.Int64("memory_shared_cache_max_percent").Title("Shared cache max").Group("Memory controller").
			Desc("memory_controller_config.shared_cache_max_percent — ceiling of the shared page cache.").
			Unit("%").Gte(0).Lte(100).Default(50),
		schemapb.Computed("memory_controller_block",
			`"memory_controller_config:\n" +
				 (("memory_hard_limit_bytes" in root) ? "  hard_limit_bytes: " + string(root.memory_hard_limit_bytes) + "\n" : "") +
				 "  soft_limit_percent: " + string(root.memory_soft_limit_percent) + "\n" +
				 "  target_utilization_percent: " + string(root.memory_target_utilization_percent) + "\n" +
				 "  shared_cache_min_percent: " + string(root.memory_shared_cache_min_percent) + "\n" +
				 "  shared_cache_max_percent: " + string(root.memory_shared_cache_max_percent)`).
			Result(schemapb.ResultString).Group("Memory controller").Title("Rendered memory_controller_config"),

		// Every repeated section (drives, hosts, pools, state storage, flags) is a
		// nested container the one-level Mustache context cannot walk, so each is
		// folded into a YAML block here and printed as one value.
		schemapb.Computed("drive_block",
			`("drives" in root) ? root.drives.map(d,
				"  - path: " + d.path + "\n    type: " + d.type.upperAscii()
			).join("\n") : "  []"`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered host_configs[].drive"),
		schemapb.Computed("host_block",
			`("hosts" in root) ? root.hosts.map(h,
				"- host: " + h.host + "\n" +
				"  node_id: " + string(h.node_id) + "\n" +
				"  host_config_id: " + string(h.host_config_id) + "\n" +
				"  port: " + string(h.port) + "\n" +
				"  location:\n" +
				"    data_center: '" + h.data_center + "'\n" +
				"    rack: '" + h.rack + "'"
			).join("\n") : "[]"`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered hosts"),
		schemapb.Computed("storage_pool_block",
			`("storage_pool_types" in root) ? root.storage_pool_types.map(p,
				"    - kind: " + p.kind + "\n" +
				"      pool_config:\n" +
				"        box_id: " + string(p.box_id) + "\n" +
				"        erasure_species: " + p.erasure_species + "\n" +
				"        kind: " + p.kind + "\n" +
				"        pdisk_filter:\n" +
				"        - property:\n" +
				"          - type: " + p.pdisk_type + "\n" +
				"        vdisk_kind: " + p.vdisk_kind
			).join("\n") : "    []"`).
			Result(schemapb.ResultString).Group("Domain").Title("Rendered storage_pool_types"),
		schemapb.Computed("state_storage_block",
			`!("state_storage_nodes" in root) || size(root.state_storage_nodes) == 0 ? "  []" :
			 "  - ssid: 1\n    ring:\n      node: [" +
			 root.state_storage_nodes.map(n, string(n)).join(", ") +
			 "]\n      nto_select: " + string(root.state_storage_nto_select)`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered state_storage"),
		// doc: configuration V1 channel_profile_config.profile[0], channels 0/1/2.
		// System tablet channels use the first configured database storage pool.
		schemapb.Computed("channel_profile_block",
			`[0, 1, 2].map(channel,
			 "    - erasure_species: " +
			 (("storage_pool_types" in root && size(root.storage_pool_types) > 0) ? root.storage_pool_types[0].erasure_species : root.static_erasure) + "\n" +
			 "      pdisk_category: " +
			 (("storage_pool_types" in root && size(root.storage_pool_types) > 0) ? (root.storage_pool_types[0].pdisk_type == "ROT" ? "0" : (root.storage_pool_types[0].pdisk_type == "NVME" ? "2" : "1")) : "1") + "\n" +
			 "      storage_pool_kind: " +
			 (("storage_pool_types" in root && size(root.storage_pool_types) > 0) ? root.storage_pool_types[0].kind : "ssd")
			).join("\n")`).
			Result(schemapb.ResultString).Group("Domain").Title("Rendered system tablet channels").
			Desc("Three channels for system tablet profile 0, derived from the first configured storage pool."),
		schemapb.Computed("feature_flag_block",
			`!("feature_flags" in root) || size(root.feature_flags) == 0 ? "" :
			 "feature_flags:\n" +
			 root.feature_flags.map(k, "  " + k + ": " + (root.feature_flags[k] ? "true" : "false")).join("\n")`).
			Result(schemapb.ResultString).Group("Feature flags").Title("Rendered feature_flags"),
		schemapb.Computed("custom_lines",
			`!("custom" in root) || size(root.custom) == 0 ? "" :
			 root.custom.map(k, k + ": " + string(root.custom[k])).join("\n")`).
			Result(schemapb.ResultString).Group("Custom").Title("Rendered extra sections"),
		schemapb.Computed("grpc_tls_block",
			`!root.grpcs ? "" :
			 "  ssl_port: " + string(root.grpc_ssl_port) + "\n" +
			 "  ca: " + root.grpc_ca + "\n" +
			 "  cert: " + root.grpc_cert + "\n" +
			 "  key: " + root.grpc_key`).
			Result(schemapb.ResultString).Group("Network").Title("Rendered grpc_config TLS keys"),
	)

	return schemapb.NewSchema(ids.Cfg("ydb.config.yaml", major)).
		Descr(fmt.Sprintf("YDB %d.x static cluster configuration (config.yaml, configuration V1).", major)).
		Strict().Coerce().
		Fields(fields...).
		Rules(
			schemapb.Rule(`!("state_storage_nodes" in root) || size(root.state_storage_nodes) == 0 || int(root.state_storage_nto_select) <= size(root.state_storage_nodes)`,
				"nto_select must not exceed the state storage ring size").ID("nto-select-le-ring"),
			schemapb.Rule(`int(root.state_storage_nto_select) % 2 == 1`,
				"nto_select must be odd").ID("nto-select-odd"),
			schemapb.Rule(`!("storage_pool_types" in root) || !("hosts" in root) ||
				root.storage_pool_types.all(p, p.erasure_species != "mirror-3-dc" || size(root.hosts) >= 9)`,
				"mirror-3-dc needs at least 9 hosts (3 per data center)").ID("mirror3dc-needs-9-hosts"),
			schemapb.Rule(`!("storage_pool_types" in root) || !("hosts" in root) ||
				root.storage_pool_types.all(p, p.erasure_species != "block-4-2" || size(root.hosts) >= 8)`,
				"block-4-2 needs at least 8 hosts").ID("block42-needs-8-hosts"),
		).
		Template("conf", fmt.Sprintf("# managed by stroppy-cloud — cfg.ydb.config.yaml@%d", major)+`
static_erasure: {{{values.static_erasure}}}
host_configs:
- host_config_id: {{{values.host_config_id}}}
  drive:
{{{values.drive_block}}}
hosts:
{{{values.host_block}}}
domains_config:
  domain:
  - name: {{{values.domain}}}
    storage_pool_types:
{{{values.storage_pool_block}}}
  state_storage:
{{{values.state_storage_block}}}
  security_config:
    enforce_user_token_requirement: {{{values.enforce_user_token_requirement}}}
blob_storage_config:
  service_set: {{{values.blob_storage_service_set}}}
channel_profile_config:
  profile:
  - profile_id: 0
    channel:
{{{values.channel_profile_block}}}
actor_system_config:
  use_auto_config: {{{values.use_auto_config}}}
  node_type: {{{values.node_type}}}
  cpu_count: {{{values.cpu_count}}}
grpc_config:
  port: {{{values.grpc_port}}}
{{#values.grpc_tls_block}}{{{values.grpc_tls_block}}}
{{/values.grpc_tls_block}}interconnect_config:
  start_tcp: true
monitoring_config:
  monitoring_port: {{{values.monitoring_port}}}
table_service_config:
  enable_query_service_spilling: {{{values.enable_query_service_spilling}}}
  spilling_service_config:
    local_file_config:
      enable: true
      root: {{{values.spilling_root}}}
      max_total_size: {{{values.spilling_max_total_size}}}
log_config:
  default_level: {{{values.log_default_level}}}
  sys_log: {{{values.log_syslog}}}
  format: {{{values.log_format}}}
{{#values.memory_controller_block}}{{{values.memory_controller_block}}}
{{/values.memory_controller_block}}{{#values.feature_flag_block}}{{{values.feature_flag_block}}}
{{/values.feature_flag_block}}{{#values.custom_lines}}{{{values.custom_lines}}}
{{/values.custom_lines}}`).
		MustBuild()
}
