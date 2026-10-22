package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Haproxy2 is cfg.haproxy.cfg@2 — the TCP load balancer in front of a
// replicated database (PostgreSQL/Patroni, MySQL, Picodata, YDB, CockroachDB:
// the `haproxy` role of every topology). Rendered as a complete haproxy.cfg.
//
// doc: https://docs.haproxy.org/2.9/configuration.html
//   - §3.1 global (maxconn, log, nbthread, stats socket)
//   - §4.2 proxies (mode, timeout connect/client/server/check, retries,
//     option tcplog, log-format, balance, bind, server, default-server)
//   - §4.3 http-check send / http-check expect — the 2.9 form. The old inline
//     `option httpchk <method> <uri> <version>` shorthand is deprecated
//     upstream, so listeners render bare `option httpchk` plus explicit
//     `http-check send` / `http-check expect` lines.
//
// Health-check URIs come from the Patroni REST API, which is what makes the
// rw/ro split work: /primary is 200 only on the leader, /replica only on a
// running replica.
// doc: https://patroni.readthedocs.io/en/latest/rest_api.html
func Haproxy2() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("haproxy.cfg", 2)).
		Descr("HAProxy 2.9 TCP balancer in front of a replicated database.").
		Strict().Coerce().
		Fields(
			// --- global ---------------------------------------------------
			schemapb.Int64("maxconn").Title("Max connections").Group("Global").
				Desc("global maxconn: process-wide connection ceiling. Must exceed the total VU count of the workload.").
				Gte(64).Lte(2000000).Default(20000),
			schemapb.Str("log").Title("Log target").Group("Global").
				Desc(`global log line, "<address> <facility> [<level>]". Empty disables logging.`).
				MaxLen(256).Default("/dev/log local0 info"),
			schemapb.Int64("nbthread").Title("Threads").Group("Global").
				Desc("global nbthread; 0 lets HAProxy bind one thread per available CPU.").
				Gte(0).Lte(256).Default(0),
			schemapb.Str("stats_socket").Title("Stats socket").Group("Global").
				Desc("global stats socket path used by the agent to read live counters. Empty disables it.").
				MaxLen(256).Default("/var/run/haproxy.sock"),

			// --- defaults -------------------------------------------------
			schemapb.Choice("mode").Title("Mode").Group("Defaults").
				Desc("defaults mode. Database wire protocols are opaque to HAProxy, so this is always tcp.").
				Opt(schemapb.StrV("tcp"), "tcp").
				Default(schemapb.StrV("tcp")).Immutable(),
			// HAProxy times are "<number><unit>" with a single unit, so these
			// are whole seconds rather than schemapb durations (whose display
			// form, 1h0m0s, HAProxy rejects).
			schemapb.Int64("timeout_connect").Title("timeout connect").Group("Defaults").
				Desc("How long a connection attempt to a backend server may take.").
				Unit("s").Gte(1).Lte(60).Default(4),
			schemapb.Int64("timeout_client").Title("timeout client").Group("Defaults").
				Desc("Inactivity allowed on the client side. Must outlast the longest workload segment or long queries are cut.").
				Unit("s").Gte(1).Lte(86400).Default(1800),
			schemapb.Int64("timeout_server").Title("timeout server").Group("Defaults").
				Desc("Inactivity allowed on the server side; mirror timeout client.").
				Unit("s").Gte(1).Lte(86400).Default(1800),
			schemapb.Int64("timeout_check").Title("timeout check").Group("Defaults").
				Desc("Timeout of one health check once the connection is established.").
				Unit("s").Gte(1).Lte(60).Default(5),
			schemapb.Int64("retries").Title("Retries").Group("Defaults").
				Desc("Connection retries against a backend server before it is declared unreachable.").
				Gte(0).Lte(10).Default(2),
			schemapb.Bool("option_tcplog").Title("option tcplog").Group("Defaults").
				Desc("Log a line per TCP session (connection times, backend, termination state).").
				Default(true),
			schemapb.Str("log_format").Title("log-format").Group("Defaults").
				Desc("Custom log-format string; empty keeps the tcplog default format.").
				MaxLen(512).Default(""),

			// --- stats ----------------------------------------------------
			schemapb.Bool("stats_enabled").Title("Stats page").Group("Stats").
				Desc("Serve the HTTP stats page on its own listener; the run detail links to it.").
				Default(true),
			schemapb.Int64("stats_port").Title("Stats port").Group("Stats").
				Desc("bind port of the stats listener.").
				When("root.stats_enabled").Gte(1).Lte(65535).Default(8404),
			schemapb.Str("stats_uri").Title("Stats URI").Group("Stats").
				Desc("stats uri path.").
				When("root.stats_enabled").Pattern(`^/[A-Za-z0-9._/-]*$`).Default("/stats"),
			schemapb.Str("stats_auth").Title("Stats auth").Group("Stats").
				Desc(`stats auth "<user>:<password>"; empty leaves the page unauthenticated inside the run network.`).
				MaxLen(128).Secret().Default(""),

			// --- listeners (cluster-filled) --------------------------------
			schemapb.List("listeners",
				schemapb.Object("",
					schemapb.Str("name").Title("Name").Group("Listeners").
						Desc("Proxy name; also the log tag.").
						Pattern(`^[A-Za-z0-9_-]+$`).Required(),
					schemapb.Int64("bind_port").Title("Bind port").Group("Listeners").
						Desc("Port this listener binds on all interfaces.").
						Gte(1).Lte(65535).Required(),
					schemapb.Choice("mode").Title("Mode").Group("Listeners").
						Desc("Proxy mode; tcp for every database protocol.").
						Opt(schemapb.StrV("tcp"), "tcp").
						Default(schemapb.StrV("tcp")),
					schemapb.Choice("balance").Title("Balance").Group("Listeners").
						Desc("Load-balancing algorithm. leastconn suits long-lived database connections; roundrobin a read fan-out; source pins a client to one node.").
						Opt(schemapb.StrV("roundrobin"), "roundrobin").
						Opt(schemapb.StrV("leastconn"), "leastconn").
						Opt(schemapb.StrV("source"), "source").
						Opt(schemapb.StrV("first"), "first").
						Default(schemapb.StrV("roundrobin")),
					schemapb.Object("check",
						schemapb.Choice("kind").Title("Check kind").Group("Listeners").
							Desc("tcp = a bare connect; httpchk = an HTTP request against the node's REST API (Patroni, and any other role-reporting endpoint).").
							Opt(schemapb.StrV("tcp"), "TCP connect").
							Opt(schemapb.StrV("httpchk"), "HTTP check").
							Default(schemapb.StrV("tcp")),
						schemapb.Choice("method").Title("Method").Group("Listeners").
							Desc("http-check send meth.").
							Opt(schemapb.StrV("GET"), "GET").
							Opt(schemapb.StrV("OPTIONS"), "OPTIONS").
							Opt(schemapb.StrV("HEAD"), "HEAD").
							Default(schemapb.StrV("GET")),
						schemapb.Str("uri").Title("URI").Group("Listeners").
							Desc("http-check send uri. Patroni: /primary is 200 only on the leader, /replica only on a running replica (optionally /replica?lag=1MB).").
							Pattern(`^/[A-Za-z0-9._/?&=%-]*$`).Default("/primary"),
						schemapb.Int64("expect_status").Title("Expected status").Group("Listeners").
							Desc("http-check expect status.").
							Gte(100).Lte(599).Default(200),
						schemapb.Int64("port").Title("Check port").Group("Listeners").
							Desc("server ... check port: the REST API port, not the database port. Patroni listens on 8008.").
							Gte(1).Lte(65535).Default(8008),
						schemapb.Int64("inter").Title("Check interval").Group("Listeners").
							Desc("default-server inter: seconds between checks.").
							Unit("s").Gte(1).Lte(60).Default(3),
						schemapb.Int64("fall").Title("Fall").Group("Listeners").
							Desc("default-server fall: failed checks before a server is taken out.").
							Gte(1).Lte(20).Default(3),
						schemapb.Int64("rise").Title("Rise").Group("Listeners").
							Desc("default-server rise: successful checks before a server is put back.").
							Gte(1).Lte(20).Default(2),
					).Strict().Title("Health check").Group("Listeners").Required(),
					schemapb.List("servers",
						schemapb.Object("",
							schemapb.Str("name").Title("Name").Group("Listeners").
								Pattern(`^[A-Za-z0-9_.-]+$`).Desc("server name in the config and in the logs.").Required(),
							schemapb.Str("address").Title("Address").Group("Listeners").
								Desc("Backend address. Filled by the server from topology.").
								MinLen(1).MaxLen(256).Required(),
							schemapb.Int64("port").Title("Port").Group("Listeners").
								Desc("Backend database port.").Gte(1).Lte(65535).Required(),
							schemapb.Int64("check_port").Title("Check port").Group("Listeners").
								Desc("This server's health-check port when it differs from the listener's (instances sharing a machine on their own ports).").
								Gte(1).Lte(65535).Nullable(),
							schemapb.Int64("weight").Title("Weight").Group("Listeners").
								Desc("server weight for the balancing algorithm.").Gte(0).Lte(256).Default(100),
							schemapb.Bool("backup").Title("Backup").Group("Listeners").
								Desc("server backup: used only when every non-backup server is down.").Default(false),
							schemapb.Int64("maxconn").Title("Max connections").Group("Listeners").
								Desc("server maxconn: per-backend connection ceiling; 0 leaves it unlimited.").
								Gte(0).Lte(1000000).Default(0),
						).Strict(),
					).Title("Servers").Group("Listeners").MinItems(1).MaxItems(64).Required(),
				).Strict(),
			).Title("Listeners").Group("Cluster").
				Desc("One frontend/backend pair per access path (rw, ro, ...). Filled by the server from topology.").
				MaxItems(16),

			// The render context is one level deep, so nested listener/server
			// blocks cannot be walked by the template: the whole proxy section
			// is assembled here and printed as a single block.
			schemapb.Computed("listener_blocks",
				`("listeners" in root) ? root.listeners.map(l,
					"listen " + l.name + "\n" +
					"    bind *:" + string(l.bind_port) + "\n" +
					"    mode " + l.mode + "\n" +
					"    balance " + l.balance + "\n" +
					(l.check.kind == "httpchk"
						? "    option httpchk\n" +
						  "    http-check send meth " + l.check.method + " uri " + l.check.uri + "\n" +
						  "    http-check expect status " + string(l.check.expect_status) + "\n"
						: "    option tcp-check\n") +
					"    default-server inter " + string(l.check.inter) +
						"s fall " + string(l.check.fall) +
						" rise " + string(l.check.rise) +
						" on-marked-down shutdown-sessions\n" +
					l.servers.map(s,
						"    server " + s.name + " " + s.address + ":" + string(s.port) +
						" check port " + string(("check_port" in s && s.check_port != null) ? s.check_port : l.check.port) +
						" weight " + string(s.weight) +
						(s.maxconn > 0 ? " maxconn " + string(s.maxconn) : "") +
						(s.backup ? " backup" : "")
					).join("\n")
				).join("\n\n") : ""`).
				Result(schemapb.ResultString).Group("Cluster").
				Title("Rendered proxy blocks").Desc("Derived: every listen block with its servers."),
			schemapb.Computed("stats_block",
				`!root.stats_enabled ? "" :
				 "listen stats\n" +
				 "    bind *:" + string(root.stats_port) + "\n" +
				 "    mode http\n" +
				 "    stats enable\n" +
				 "    stats uri " + root.stats_uri + "\n" +
				 "    stats refresh 5s" +
				 (root.stats_auth == "" ? "" : "\n    stats auth " + root.stats_auth)`).
				Result(schemapb.ResultString).Group("Stats").Title("Rendered stats block"),
			schemapb.Computed("global_extra",
				`(root.log == "" ? "" : "    log " + root.log + "\n") +
				 (root.nbthread == 0 ? "" : "    nbthread " + string(root.nbthread) + "\n") +
				 (root.stats_socket == "" ? "" : "    stats socket " + root.stats_socket + " mode 660 level admin\n")`).
				Result(schemapb.ResultString).Group("Global").Title("Rendered optional global lines"),
			schemapb.Computed("defaults_extra",
				`(root.log == "" ? "" : "    log global\n") +
				 (root.option_tcplog ? "    option tcplog\n" : "") +
				 (root.log_format == "" ? "" : "    log-format " + root.log_format + "\n")`).
				Result(schemapb.ResultString).Group("Defaults").Title("Rendered optional defaults lines"),
		).
		Rules(
			schemapb.Rule(`!("listeners" in root) || root.listeners.all(l, l.bind_port != root.stats_port) || !root.stats_enabled`,
				"a listener cannot bind the stats port").ID("listener-vs-stats-port"),
			schemapb.Rule(`!("listeners" in root) || root.listeners.all(l, size(l.servers) <= int(root.maxconn))`,
				"global maxconn must cover at least one connection per backend server").ID("maxconn-covers-servers"),
		).
		Template("conf", `# managed by stroppy-cloud — cfg.haproxy.cfg@2
global
    maxconn {{{values.maxconn}}}
{{{values.global_extra}}}
defaults
    mode {{{values.mode}}}
    retries {{{values.retries}}}
    timeout connect {{{values.timeout_connect}}}s
    timeout client {{{values.timeout_client}}}s
    timeout server {{{values.timeout_server}}}s
    timeout check {{{values.timeout_check}}}s
{{{values.defaults_extra}}}
{{{values.listener_blocks}}}

{{{values.stats_block}}}
`).
		MustBuild()
}
