package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Picodata25 is cfg.picodata.yaml@25 — the picodata.yaml an instance is
// started with (`picodata run --config`), in the 25.3 shape.
//
// doc: https://docs.picodata.io/picodata/25.3/reference/config/
func Picodata25() *schemapb.Schema { return picodataYaml(25, 0) }

// Picodata26 is cfg.picodata.yaml@26 — the 26.1/26.2 shape.
//
// doc: https://docs.picodata.io/picodata/26.1/reference/config/
// doc: https://docs.picodata.io/picodata/26.2/reference/config/
func Picodata26() *schemapb.Schema { return picodataYaml(26, 0) }

// Picodata261 uses the 26.1 network layout without the tier modes introduced in 26.2.
// doc: https://docs.picodata.io/picodata/26.1/reference/config/
func Picodata261() *schemapb.Schema { return picodataYaml(26, 1) }

// picodataYaml builds cfg.picodata.yaml@<major>. Picodata knows exactly two
// top-level sections, cluster and instance; everything else is a CLI override.
//
// Common to both majors (checked against the 25.3 and 26.2 references):
//   - the replica set key is replicaset_name (underscore);
//   - instance.memtx exposes memory and max_tuple_size — the tarantool
//     checkpoint_count / checkpoint_interval knobs are not part of the
//     configuration file and are therefore not modeled;
//   - memory sizes are integers or K/M/G/T suffixed strings, 1K = 1024.
//
// What 26.x changed in the file (this is why the major exists):
//
//	iproto_listen / iproto_advertise  ->  instance.iproto.{listen,advertise}
//	http_listen                       ->  instance.http.listen
//	instance.pg.{listen,advertise}    ->  instance.pgproto.{listen,advertise}
//	instance.pg.ssl                   ->  instance.pgproto.tls.enabled
//	new: cluster.tier.<t>.replication_mode, cluster.tier.<t>.wal_mode
//	new: instance.memtx.system_memory, instance.wal_dir, instance.backup_dir
//
//nolint:funlen,maintidx // one flat parameter table plus its template
func picodataYaml(major, minor uint64) *schemapb.Schema {
	is26 := major >= 26

	tierFields := []schemapb.FieldDef{
		schemapb.Str("name").Title("Tier name").Group("Cluster").
			Desc(`Tier key under cluster.tier. One tier must be named "default".`).
			Pattern(`^[a-z][a-z0-9_-]{0,31}$`).Required(),
		schemapb.Int64("replication_factor").Title("Replication factor").Group("Cluster").
			Desc("cluster.tier.<name>.replication_factor — instances per replicaset in this tier.").
			Gte(1).Lte(32).Default(1),
		schemapb.Bool("can_vote").Title("Can vote").Group("Cluster").
			Desc("cluster.tier.<name>.can_vote — whether instances of this tier take part in raft voting.").
			Default(true),
		schemapb.Int64("bucket_count").Title("Bucket count").Group("Cluster").
			Desc("cluster.tier.<name>.bucket_count — sharding buckets of this tier.").
			Gte(1).Lte(1000000).Default(3000),
	}

	if is26 && minor != 1 {
		tierFields = append(tierFields,
			// doc: 26.2 reference/config — cluster.tier.<name>.replication_mode, default async
			schemapb.Choice("replication_mode").Title("Replication mode").Group("Cluster").
				Desc("cluster.tier.<name>.replication_mode — 26.x only: sync makes a write wait for the replicas of the replicaset.").
				Opt(schemapb.StrV("async"), "async").
				Opt(schemapb.StrV("sync"), "sync").
				Default(schemapb.StrV("async")),
			// doc: 26.2 reference/config — cluster.tier.<name>.wal_mode, default write
			schemapb.Choice("wal_mode").Title("WAL mode").Group("Cluster").
				Desc("cluster.tier.<name>.wal_mode — 26.x only: fsync makes every WAL write durable before the transaction returns.").
				Opt(schemapb.StrV("write"), "write").
				Opt(schemapb.StrV("fsync"), "fsync").
				Default(schemapb.StrV("write")),
		)
	}

	fields := []schemapb.FieldDef{
		// --- cluster --------------------------------------------------
		schemapb.Str("cluster_name").Title("Cluster name").Group("Cluster").
			Desc("cluster.name — every instance of one cluster must agree on it (upstream default demo).").
			Pattern(`^[A-Za-z0-9_-]{1,63}$`).Default("stroppy"),
		schemapb.Int64("default_replication_factor").Title("Default replication factor").Group("Cluster").
			Desc("cluster.default_replication_factor — replicas per replicaset for tiers that do not set their own (upstream default 1).").
			Gte(1).Lte(32).Default(1),
		schemapb.Int64("default_bucket_count").Title("Default bucket count").Group("Cluster").
			Desc("cluster.default_bucket_count — sharding buckets per tier (upstream default 3000).").
			Gte(1).Lte(1000000).Default(3000),
		schemapb.Bool("shredding").Title("Shredding").Group("Cluster").
			Desc("cluster.shredding — overwrite deleted data files instead of unlinking them. Costs write bandwidth.").
			Default(false),
		schemapb.List("tiers", schemapb.Object("", tierFields...).Strict()).
			Title("Tiers").Group("Cluster").
			Desc("cluster.tier map. Filled by the server from the picodata params (tiers[] of the database form).").
			MaxItems(16),

		// --- instance identity (cluster-filled) ------------------------
		schemapb.Str("instance_name").Title("Instance name").Group("Cluster").
			Desc("instance.name. Filled by the server from topology; unset lets picodata derive tier_replicaset_instance.").
			Pattern(`^[A-Za-z0-9_-]{1,63}$`).Nullable(),
		schemapb.Str("replicaset_name").Title("Replicaset name").Group("Cluster").
			Desc("instance.replicaset_name. Filled by the server from topology.").
			Pattern(`^[A-Za-z0-9_-]{1,63}$`).Nullable(),
		schemapb.Str("tier").Title("Tier").Group("Cluster").
			Desc("instance.tier — which tier this instance joins. Filled by the server from topology.").
			Pattern(`^[a-z][a-z0-9_-]{0,31}$`).Nullable(),
		schemapb.List("peer", schemapb.Str("").Pattern(`^[^\s]+:[0-9]{1,5}$`)).
			Title("Peers").Group("Cluster").
			Desc(`instance.peer — "host:port" of the instances used to join the cluster. Filled by the server from topology.`).
			MaxItems(64).Unique(),
		schemapb.MapOf("failure_domain", schemapb.Str("value").Pattern(`^[A-Za-z0-9._-]{1,63}$`)).
			Title("Failure domain").Group("Cluster").
			Desc("instance.failure_domain — arbitrary key/value describing where the instance runs; picodata spreads a replicaset across distinct values. Filled by the server from topology.").
			MaxEntries(8),

		// --- instance runtime ------------------------------------------
		schemapb.Str("instance_dir").Title("Instance directory").Group("Instance").
			Desc("instance.instance_dir — snapshots, xlogs and the TLS material live here; point it at the data disk.").
			Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/var/lib/picodata"),
		schemapb.Str("iproto_listen").Title("iproto listen").Group("Network").
			Desc(picodataDesc(is26, "instance.iproto_listen", "instance.iproto.listen") +
				" — the binary protocol socket other instances connect to.").
			Pattern(`^[^\s]+:[0-9]{1,5}$`).Default("0.0.0.0:3301"),
		schemapb.Str("iproto_advertise").Title("iproto advertise").Group("Cluster").
			Desc(picodataDesc(is26, "instance.iproto_advertise", "instance.iproto.advertise") +
				" — the address other instances should use. Filled by the server from topology.").
			Pattern(`^[^\s]+:[0-9]{1,5}$`).Nullable(),
		schemapb.Str("http_listen").Title("HTTP listen").Group("Network").
			Desc(picodataDesc(is26, "instance.http_listen", "instance.http.listen") +
				" — the HTTP endpoint (webui, metrics). Empty disables it.").
			MaxLen(128).Default("0.0.0.0:8081"),
		schemapb.Str("pg_listen").Title("PostgreSQL listen").Group("Network").
			Desc(picodataDesc(is26, "instance.pg.listen", "instance.pgproto.listen") +
				" — the PostgreSQL wire-protocol port stroppy connects to.").
			Pattern(`^[^\s]+:[0-9]{1,5}$`).Default("0.0.0.0:4327"),
		schemapb.Str("pg_advertise").Title("PostgreSQL advertise").Group("Cluster").
			Desc(picodataDesc(is26, "instance.pg.advertise", "instance.pgproto.advertise") +
				". Filled by the server from topology.").
			Pattern(`^[^\s]+:[0-9]{1,5}$`).Nullable(),
		schemapb.Bool("pg_ssl").Title("PostgreSQL SSL").Group("Network").
			Desc(picodataDesc(is26, "instance.pg.ssl", "instance.pgproto.tls.enabled") +
				" — requires server.crt/server.key inside instance_dir.").
			Default(false),
		schemapb.Str("admin_socket").Title("Admin socket").Group("Instance").
			Desc("instance.admin_socket — the unix socket `picodata admin` attaches to.").
			Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/var/run/picodata/admin.sock"),
		schemapb.Int64("boot_timeout").Title("Boot timeout").Group("Instance").
			Desc("instance.boot_timeout — seconds an instance waits to join the cluster before giving up (upstream default 7200).").
			Unit("s").Gte(1).Lte(86400).Default(7200),
		schemapb.Str("audit").Title("Audit destination").Group("Instance").
			Desc(`instance.audit — "file:<path>", "pipe:<command>" or "syslog:". Empty disables the audit log.`).
			Pattern(`^$|^(file:|pipe:|syslog:).*$`).Default(""),

		// --- memtx / vinyl ---------------------------------------------
		schemapb.Str("memtx_memory").Title("memtx memory").Group("Storage").
			Desc("instance.memtx.memory — the in-memory storage arena; minimum 32M. The product default is 2G (upstream 64M is far too small for a benchmark).").
			Pattern(`^[0-9]+[KMGT]?$`).Default("2G"),
		schemapb.Str("memtx_max_tuple_size").Title("memtx max tuple size").Group("Storage").
			Desc("instance.memtx.max_tuple_size — largest single tuple (upstream default 1M).").
			Pattern(`^[0-9]+[KMGT]?$`).Default("1M"),
		schemapb.Str("vinyl_memory").Title("vinyl memory").Group("Storage").
			Desc("instance.vinyl.memory — write buffer of the on-disk engine (upstream default 128M).").
			Pattern(`^[0-9]+[KMGT]?$`).Default("128M"),
		schemapb.Str("vinyl_cache").Title("vinyl cache").Group("Storage").
			Desc("instance.vinyl.cache — read cache of the on-disk engine (upstream default 128M).").
			Pattern(`^[0-9]+[KMGT]?$`).Default("128M"),
		schemapb.Int64("vinyl_read_threads").Title("vinyl read threads").Group("Storage").
			Desc("instance.vinyl.read_threads (upstream default 1).").
			Gte(1).Lte(64).Default(1),
		schemapb.Int64("vinyl_write_threads").Title("vinyl write threads").Group("Storage").
			Desc("instance.vinyl.write_threads (upstream default 4).").
			Gte(2).Lte(64).Default(4),

		// --- log --------------------------------------------------------
		schemapb.Choice("log_level").Title("Log level").Group("Logging").
			Desc("instance.log.level.").
			Opt(schemapb.StrV("fatal"), "fatal").
			Opt(schemapb.StrV("system"), "system").
			Opt(schemapb.StrV("error"), "error").
			Opt(schemapb.StrV("crit"), "crit").
			Opt(schemapb.StrV("warn"), "warn").
			Opt(schemapb.StrV("info"), "info").
			Opt(schemapb.StrV("verbose"), "verbose").
			Opt(schemapb.StrV("debug"), "debug").
			Default(schemapb.StrV("info")),
		schemapb.Choice("log_format").Title("Log format").Group("Logging").
			Desc("instance.log.format.").
			Opt(schemapb.StrV("plain"), "plain").
			Opt(schemapb.StrV("json"), "json").
			Default(schemapb.StrV("plain")),
		schemapb.Str("log_destination").Title("Log destination").Group("Logging").
			Desc(`instance.log.destination — "file:<path>", "pipe:<command>" or "syslog:". Empty logs to stderr, which is what the agent collects.`).
			Pattern(`^$|^(file:|pipe:|syslog:).*$`).Default(""),

		// --- escape hatch ------------------------------------------------
		schemapb.MapOf("custom", schemapb.Str("value").MaxLen(256)).
			Title("Extra instance keys").Group("Custom").
			Desc("Further instance.* keys, emitted verbatim at the end of the instance section as `key: value`.").
			MaxEntries(32).
			Rules(schemapb.Rule(
				`this.all(k, k.matches("^[a-z][a-z0-9_]*$"))`,
				"instance keys are lower_snake identifiers").ID("custom-key-shape")),
	}

	if is26 {
		fields = append(fields,
			// doc: 26.2 reference/config — instance.memtx.system_memory, default 256M
			schemapb.Str("memtx_system_memory").Title("memtx system memory").Group("Storage").
				Desc("instance.memtx.system_memory — 26.x only: the arena picodata reserves for its own system spaces (upstream default 256M).").
				Pattern(`^[0-9]+[KMGT]?$`).Default("256M"),
			// doc: 26.2 reference/config — instance.wal_dir, defaults to instance_dir
			schemapb.Str("wal_dir").Title("WAL directory").Group("Instance").
				Desc("instance.wal_dir — 26.x only: where xlogs are written; separate it from instance_dir to put the WAL on its own disk.").
				Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/var/lib/picodata"),
			// doc: 26.2 reference/config — instance.backup_dir, defaults to <instance_dir>/backup
			schemapb.Str("backup_dir").Title("Backup directory").Group("Instance").
				Desc("instance.backup_dir — 26.x only: destination of `picodata backup`.").
				Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/var/lib/picodata/backup"),
		)
	}

	// The tier map, the peer list and the failure domain map are nested
	// containers: the Mustache context has one level, so they are folded into
	// YAML blocks here.
	fields = append(fields,
		schemapb.Computed("tier_block", picodataTierBlock(is26 && minor != 1)).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered cluster.tier"),
		schemapb.Computed("peer_line",
			`!("peer" in root) || size(root.peer) == 0 ? "" :
			 "  peer: [" + root.peer.map(p, '"' + p + '"').join(", ") + "]"`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered instance.peer"),
		schemapb.Computed("failure_domain_line",
			`!("failure_domain" in root) || size(root.failure_domain) == 0 ? "" :
			 "  failure_domain: {" + root.failure_domain.map(k, k + ": " + string(root.failure_domain[k])).join(", ") + "}"`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered instance.failure_domain"),
		schemapb.Computed("custom_lines",
			`!("custom" in root) || size(root.custom) == 0 ? "" :
			 root.custom.map(k, "  " + k + ": " + string(root.custom[k])).join("\n")`).
			Result(schemapb.ResultString).Group("Custom").Title("Rendered extra instance keys"),
		schemapb.Computed("identity_lines", picodataIdentityLines(is26)).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered instance identity"),
		schemapb.Computed("pg_advertise_line",
			`("pg_advertise" in root) ? "    advertise: " + root.pg_advertise : ""`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered the pg advertise address"),
	)

	if is26 {
		fields = append(fields,
			schemapb.Computed("iproto_advertise_line",
				`("iproto_advertise" in root) ? "    advertise: " + root.iproto_advertise : ""`).
				Result(schemapb.ResultString).Group("Cluster").Title("Rendered instance.iproto.advertise"),
		)
	}

	identity := ids.Cfg("picodata.yaml", major)
	if minor != 0 {
		identity = ids.CfgMinor("picodata.yaml", major, minor)
	}
	return schemapb.NewSchema(identity).
		Descr(picodataDescr(is26)).
		Strict().Coerce().
		Fields(fields...).
		Rules(
			schemapb.Rule(`!("tiers" in root) || size(root.tiers) == 0 || root.tiers.exists(t, t.name == "default")`,
				`one tier must be named "default"`).ID("default-tier-required"),
			schemapb.Rule(`!("tier" in root) || !("tiers" in root) || size(root.tiers) == 0 || root.tiers.exists(t, t.name == root.tier)`,
				"instance.tier must name one of the declared tiers").ID("instance-tier-declared"),
		).
		Template("conf", picodataTemplate(is26)).
		MustBuild()
}

// picodataDesc picks the YAML path the major actually uses.
func picodataDesc(is26 bool, path25, path26 string) string {
	if is26 {
		return path26
	}

	return path25
}

func picodataDescr(is26 bool) string {
	if is26 {
		return "Picodata 26.x instance configuration (picodata.yaml): nested iproto/http/pgproto sections."
	}

	return "Picodata 25.3 instance configuration (picodata.yaml)."
}

func picodataTierBlock(is26 bool) string {
	extra, fallback := "", `""`

	if is26 {
		fallback = `"\n      replication_mode: async\n      wal_mode: write"`
		extra = `+ "\n      replication_mode: " + t.replication_mode +
					"\n      wal_mode: " + t.wal_mode`
	}

	return `("tiers" in root) ? root.tiers.map(t,
					"    " + t.name + ":\n" +
					"      replication_factor: " + string(t.replication_factor) + "\n" +
					"      can_vote: " + (t.can_vote ? "true" : "false") + "\n" +
					"      bucket_count: " + string(t.bucket_count) ` + extra + `
				).join("\n") : "    default:\n      replication_factor: " + string(root.default_replication_factor) + ` + fallback
}

// picodataIdentityLines renders the flat instance.* identity keys. On 25.3
// iproto_advertise is one of them; on 26.x it moved under instance.iproto and
// is rendered by iproto_advertise_line instead.
func picodataIdentityLines(is26 bool) string {
	base := `(("instance_name" in root) ? "  name: " + root.instance_name + "\n" : "") +
				 (("replicaset_name" in root) ? "  replicaset_name: " + root.replicaset_name + "\n" : "") +
				 (("tier" in root) ? "  tier: " + root.tier + "\n" : "")`
	if is26 {
		return base
	}

	return base + ` +
				 (("iproto_advertise" in root) ? "  iproto_advertise: " + root.iproto_advertise + "\n" : "")`
}

func picodataTemplate(is26 bool) string {
	head := `# managed by stroppy-cloud — cfg.picodata.yaml
cluster:
  name: {{{values.cluster_name}}}
  default_replication_factor: {{{values.default_replication_factor}}}
  default_bucket_count: {{{values.default_bucket_count}}}
  shredding: {{{values.shredding}}}
  tier:
{{{values.tier_block}}}
instance:
{{{values.identity_lines}}}  instance_dir: {{{values.instance_dir}}}
`

	tail := `  admin_socket: {{{values.admin_socket}}}
  boot_timeout: {{{values.boot_timeout}}}
{{#values.failure_domain_line}}{{{values.failure_domain_line}}}
{{/values.failure_domain_line}}  memtx:
    memory: {{{values.memtx_memory}}}
`

	if !is26 {
		return head + `  iproto_listen: {{{values.iproto_listen}}}
{{#values.peer_line}}{{{values.peer_line}}}
{{/values.peer_line}}  http_listen: {{{values.http_listen}}}
  pg:
    listen: {{{values.pg_listen}}}
{{#values.pg_advertise_line}}{{{values.pg_advertise_line}}}
{{/values.pg_advertise_line}}    ssl: {{{values.pg_ssl}}}
` + tail + `    max_tuple_size: {{{values.memtx_max_tuple_size}}}
` + picodataVinylLog()
	}

	return head + `  wal_dir: {{{values.wal_dir}}}
  backup_dir: {{{values.backup_dir}}}
  iproto:
    listen: {{{values.iproto_listen}}}
{{#values.iproto_advertise_line}}{{{values.iproto_advertise_line}}}
{{/values.iproto_advertise_line}}{{#values.peer_line}}{{{values.peer_line}}}
{{/values.peer_line}}  http:
    listen: {{{values.http_listen}}}
  pgproto:
    listen: {{{values.pg_listen}}}
{{#values.pg_advertise_line}}{{{values.pg_advertise_line}}}
{{/values.pg_advertise_line}}    tls:
      enabled: {{{values.pg_ssl}}}
` + tail + `    system_memory: {{{values.memtx_system_memory}}}
    max_tuple_size: {{{values.memtx_max_tuple_size}}}
` + picodataVinylLog()
}

func picodataVinylLog() string {
	return `  vinyl:
    memory: {{{values.vinyl_memory}}}
    cache: {{{values.vinyl_cache}}}
    read_threads: {{{values.vinyl_read_threads}}}
    write_threads: {{{values.vinyl_write_threads}}}
  log:
    level: {{{values.log_level}}}
    format: {{{values.log_format}}}
{{#values.log_destination}}    destination: {{{values.log_destination}}}
{{/values.log_destination}}{{#values.audit}}  audit: {{{values.audit}}}
{{/values.audit}}{{#values.custom_lines}}{{{values.custom_lines}}}
{{/values.custom_lines}}`
}
