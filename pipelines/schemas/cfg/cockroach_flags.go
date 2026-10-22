package cfg

import (
	"fmt"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Cockroach24 is cfg.cockroach.flags@24 (the v24.1 / v24.3 LTS lines).
//
// doc: https://docs.cockroachlabs.com/docs/v24.1/cockroach-start
// doc: https://docs.cockroachlabs.com/docs/v24.3/cockroach-start
func Cockroach24() *schemapb.Schema { return cockroachFlags(24) }

// Cockroach25 is cfg.cockroach.flags@25 (the v25.2 / v25.4 LTS lines).
//
// doc: https://docs.cockroachlabs.com/docs/v25.4/cockroach-start
func Cockroach25() *schemapb.Schema { return cockroachFlags(25) }

// Cockroach26 is cfg.cockroach.flags@26 (the v26.2 / v26.3 lines).
//
// doc: https://docs.cockroachlabs.com/docs/v26.3/cockroach-start
func Cockroach26() *schemapb.Schema { return cockroachFlags(26) }

// cockroachFlags builds cfg.cockroach.flags@<major>. CockroachDB has no
// configuration file: a node is configured entirely by `cockroach start` flags
// plus cluster settings applied over SQL once the cluster is live. The schema
// therefore carries two templates:
//
//	"conf"     the flags line appended to `cockroach start`
//	"settings" the SET CLUSTER SETTING block run against the first node
//
// doc: https://docs.cockroachlabs.com/docs/v26.3/cockroach-start
// doc: https://docs.cockroachlabs.com/docs/v26.3/cluster-settings
//
// Checked across v24.1, v24.3, v25.4 and v26.3: not one of the flags below
// was removed, renamed or deprecated — the majors share the whole surface.
// --wal-failover (preview in 24.1), --locality-advertise-addr and
// --max-disk-temp-storage (default 32GiB) are already on the v24.1 page.
// What did change: the --cache upstream default went 128MiB (24.x) ->
// 256MiB (25.4+); the schema pins 25% on every major, so the change is
// documentation only.
//
// The deprecated networking flags (--host, --port, --advertise-host,
// --http-host, --http-port) are deliberately absent on every major; only the
// --*-addr forms are modeled.
//
//nolint:funlen // one flat flag table plus its templates
func cockroachFlags(major uint64) *schemapb.Schema {
	fields := []schemapb.FieldDef{
		// --- store ----------------------------------------------------
		// doc: --store path=,size=,attrs=,ballast-size= (default path cockroach-data)
		schemapb.Str("store_path").Title("Store path").Group("Store").
			Desc("--store path= : the data directory, on the mounted data disk.").
			Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/var/lib/cockroach/data"),
		schemapb.Str("store_size").Title("Store size").Group("Store").
			Desc("--store size= : maximum store size as a percentage (25%) or a size (100GiB). Upstream default is 100%.").
			Pattern(`^([0-9]{1,3}%|[0-9]+(\.[0-9]+)?(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)?)$`).Default("90%"),
		schemapb.Str("ballast_size").Title("Ballast size").Group("Store").
			Desc("--store ballast-size= : a reserved file that can be deleted to recover a full disk; 0 disables it.").
			Pattern(`^([0-9]{1,3}%|[0-9]+(\.[0-9]+)?(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)?)$`).Default("0"),

		// --- memory ---------------------------------------------------
		// doc: --cache default 128MiB on 24.x and 256MiB from 25.4;
		// --max-sql-memory default 25%; --max-tsdb-memory default
		// max(1%, 64MiB). Docs: the three together should stay under 75% of RAM.
		schemapb.Str("cache").Title("Cache").Group("Memory").
			Desc("--cache : block cache, a percentage or a size. The upstream default (128MiB up to 24.x, 256MiB from 25.4) is far too small for a benchmark node.").
			Pattern(`^([0-9]{1,3}%|0?\.[0-9]+|[0-9]+(\.[0-9]+)?(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)?)$`).Default("25%"),
		schemapb.Str("max_sql_memory").Title("Max SQL memory").Group("Memory").
			Desc("--max-sql-memory : budget for SQL execution (sorts, joins, results).").
			Pattern(`^([0-9]{1,3}%|0?\.[0-9]+|[0-9]+(\.[0-9]+)?(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)?)$`).Default("25%"),
		schemapb.Str("max_tsdb_memory").Title("Max TSDB memory").Group("Memory").
			Desc("--max-tsdb-memory : budget for the built-in time-series store that feeds the DB Console.").
			Pattern(`^([0-9]{1,3}%|0?\.[0-9]+|[0-9]+(\.[0-9]+)?(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)?)$`).Default("64MiB"),

		// --- topology (cluster-filled) ---------------------------------
		// doc: --locality is an ordered list of key=value pairs; the keys
		// and their order must be identical on every node.
		schemapb.Str("locality").Title("Locality").Group("Cluster").
			Desc(`--locality : ordered "key=value,key=value" pairs describing where the node runs. Filled by the server from topology (region/zone of the machine).`).
			Pattern(`^[a-z][a-z0-9_-]*=[A-Za-z0-9._-]+(,[a-z][a-z0-9_-]*=[A-Za-z0-9._-]+)*$`).Nullable().
			Examples(schemapb.StrV("region=ru-central1,zone=ru-central1-a")),
		schemapb.List("join", schemapb.Str("").MinLen(1).MaxLen(256)).
			Title("Join addresses").Group("Cluster").
			Desc("--join : the addresses of the other nodes. Filled by the server from topology; identical on every node.").
			MaxItems(64).Unique(),
		schemapb.Str("advertise_addr").Title("Advertise address").Group("Cluster").
			Desc("--advertise-addr : how other nodes and clients reach this one. Filled by the server from topology.").
			MaxLen(256).Nullable(),
		schemapb.Str("listen_addr").Title("Listen address").Group("Network").
			Desc("--listen-addr : interface and port for inter-node and SQL traffic.").
			MaxLen(256).Default("0.0.0.0:26257"),
		schemapb.Str("http_addr").Title("HTTP address").Group("Network").
			Desc("--http-addr : DB Console and /_status/vars (the Prometheus endpoint the run scrapes).").
			MaxLen(256).Default("0.0.0.0:8080"),
		schemapb.Str("sql_addr").Title("SQL address").Group("Network").
			Desc("--sql-addr : a separate listener for client SQL; empty shares --listen-addr.").
			MaxLen(256).Default(""),

		// --- security -------------------------------------------------
		schemapb.Bool("insecure").Title("Insecure").Group("Security").
			Desc("--insecure : no TLS, no authentication. Acceptable only inside the run's private network, which is where these stands live. Must stay on: the stand provisions no node or CA certificates.").
			Default(true),
		schemapb.Str("certs_dir").Title("Certificates directory").Group("Security").
			Desc("--certs-dir : node and CA certificates; read only in secure mode.").
			When("!root.insecure").
			Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/etc/cockroach/certs"),

		// --- cluster identity / clock ---------------------------------
		schemapb.Duration("max_offset").Title("Max clock offset").Group("Cluster settings").
			Desc("--max-offset : tolerated clock skew between nodes; a node that exceeds it kills itself. Must be identical on all nodes (upstream default 500ms).").
			Gte(50 * time.Millisecond).Lte(10 * time.Second).Default(500 * time.Millisecond),
		schemapb.Str("cluster_name").Title("Cluster name").Group("Cluster settings").
			Desc("--cluster-name : guards against a node joining the wrong cluster; must match on every node.").
			Pattern(`^[a-z0-9][a-z0-9-]{0,62}$`).Default("stroppy"),

		// --- logging --------------------------------------------------
		// doc: --log takes a YAML configuration; the individual logging
		// flags cannot be combined with it.
		schemapb.Str("log").Title("Log configuration").Group("Logging").
			Desc("--log : the logging configuration as a YAML string. Empty leaves the built-in default (file logs under the store).").
			MaxLen(4096).
			Default(`{sinks: {stderr: {channels: all, filter: WARNING, redact: false}}}`),

		// --- cluster settings -----------------------------------------
		schemapb.MapOf("cluster_settings", schemapb.Str("value").MaxLen(256)).
			Title("Cluster settings").Group("Cluster settings").
			Desc("SET CLUSTER SETTING <name> = <value>, applied once against the first node after the cluster is initialized.").
			MaxEntries(64).
			Rules(schemapb.Rule(
				`this.all(k, k.matches("^[a-z][a-z0-9_]*([.][a-z0-9_]+)+$"))`,
				"cluster setting names look like kv.snapshot_rebalance.max_rate").ID("setting-name-shape")),
		// doc: https://docs.cockroachlabs.com/docs/stable/configure-replication-zones — gc.ttlseconds
		schemapb.Int64("gc_ttl_seconds").Title("GC TTL of the default range").Group("Cluster settings").
			Desc("ALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds : how long overwritten MVCC values are kept. Empty keeps the upstream zone (4 hours).").
			Unit("s").Gte(60).Lte(7 * 24 * 3600).Nullable(),

		// doc: v24.1..v26.3 cockroach-start — --wal-failover
		schemapb.Choice("wal_failover").Title("WAL failover").Group("Store").
			Desc("--wal-failover : when a store's disk stalls, keep writing its WAL to another store instead of blocking. Needs at least two stores; disabled on a single-store node.").
			Opt(schemapb.StrV("disabled"), "disabled").
			Opt(schemapb.StrV("among-stores"), "among-stores").
			Default(schemapb.StrV("disabled")),
		// doc: v24.1..v26.3 cockroach-start — --locality-advertise-addr
		schemapb.Str("locality_advertise_addr").Title("Locality advertise address").Group("Cluster").
			Desc(`--locality-advertise-addr : "key=value@address" pairs letting nodes in the same locality use a private address. Filled by the server from topology.`).
			MaxLen(512).Nullable().
			Examples(schemapb.StrV("region=ru-central1@10.0.0.11:26257")),
		// doc: v24.1..v26.3 cockroach-start — --max-disk-temp-storage, default 32GiB
		schemapb.Str("max_disk_temp_storage").Title("Max disk temp storage").Group("Store").
			Desc("--max-disk-temp-storage : disk budget queries may spill to. Upstream default 32GiB; it is not counted towards the store, so --store size plus this must fit on the disk.").
			Pattern(`^([0-9]{1,3}%|[0-9]+(\.[0-9]+)?(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)?)$`).Default("32GiB"),
	}

	// Lists and maps are invisible to a one-level Mustache context, so
	// the repeated flags and the SQL block are assembled here.
	fields = append(fields,
		schemapb.Computed("join_flag",
			`!("join" in root) || size(root.join) == 0 ? "" : " --join=" + root.join.join(",")`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered --join"),
		schemapb.Computed("locality_flag",
			`("locality" in root) ? " --locality=" + root.locality : ""`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered --locality"),
		schemapb.Computed("advertise_flag",
			`("advertise_addr" in root) ? " --advertise-addr=" + root.advertise_addr : ""`).
			Result(schemapb.ResultString).Group("Cluster").Title("Rendered --advertise-addr"),
		schemapb.Computed("sql_addr_flag",
			`root.sql_addr == "" ? "" : " --sql-addr=" + root.sql_addr`).
			Result(schemapb.ResultString).Group("Network").Title("Rendered --sql-addr"),
		schemapb.Computed("security_flag",
			`root.insecure ? " --insecure" : " --certs-dir=" + root.certs_dir`).
			Result(schemapb.ResultString).Group("Security").Title("Rendered security flag"),
		schemapb.Computed("store_flag",
			`" --store=path=" + root.store_path + ",size=" + root.store_size +
				 (root.ballast_size == "0" ? "" : ",ballast-size=" + root.ballast_size)`).
			Result(schemapb.ResultString).Group("Store").Title("Rendered --store"),
		schemapb.Computed("log_flag",
			`root.log == "" ? "" : " --log=" + root.log`).
			Result(schemapb.ResultString).Group("Logging").Title("Rendered --log"),
		schemapb.Computed("settings_sql",
			`(("cluster_settings" in root)
					? root.cluster_settings.map(k, "SET CLUSTER SETTING " + k + " = " + string(root.cluster_settings[k]) + ";").join("\n")
					: "") +
			 (("gc_ttl_seconds" in root) ? "\nALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds = " + string(root.gc_ttl_seconds) + ";" : "")`).
			Result(schemapb.ResultString).Group("Cluster settings").
			Title("Rendered SQL").Desc("Derived: the SET CLUSTER SETTING block and the default zone."),
		schemapb.Computed("modern_flags",
			`(root.wal_failover == "disabled" ? "" : " --wal-failover=" + root.wal_failover) +
			 " --max-disk-temp-storage=" + root.max_disk_temp_storage +
			 (("locality_advertise_addr" in root) ? " --locality-advertise-addr=" + root.locality_advertise_addr : "")`).
			Result(schemapb.ResultString).Group("Store").
			Title("Rendered store and locality flags").
			Desc("Derived: --wal-failover, --max-disk-temp-storage and --locality-advertise-addr."),
	)

	return schemapb.NewSchema(ids.Cfg("cockroach.flags", major)).
		Descr(fmt.Sprintf("CockroachDB %d.x node start flags and cluster settings.", major)).
		Strict().Coerce().
		Fields(fields...).
		Rules(
			// Secure mode needs node, client and CA certificates in certs_dir
			// on every node and the runner; the stand creates none, so a
			// secure node could not start or be reached.
			// doc: https://docs.cockroachlabs.com/docs/stable/cockroach-cert
			schemapb.Rule("root.insecure",
				"secure mode is not available: the stand provisions no node or CA certificates").ID("insecure-only"),
		).
		Template("conf", `--listen-addr={{{values.listen_addr}}}{{{values.advertise_flag}}}{{{values.sql_addr_flag}}} --http-addr={{{values.http_addr}}}{{{values.store_flag}}} --cache={{{values.cache}}} --max-sql-memory={{{values.max_sql_memory}}} --max-tsdb-memory={{{values.max_tsdb_memory}}} --max-offset={{{values.max_offset}}} --cluster-name={{{values.cluster_name}}}{{{values.locality_flag}}}{{{values.join_flag}}}{{{values.security_flag}}}{{{values.modern_flags}}}{{{values.log_flag}}}
`).
		Template("settings", fmt.Sprintf(`-- managed by stroppy-cloud — cfg.cockroach.flags@%d
{{{values.settings_sql}}}
`, major)).
		MustBuild()
}
