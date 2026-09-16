package cfg

import (
	"fmt"
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// PatroniYml3 is cfg.patroni.yml@3 — the Patroni 3.x node configuration.
//
// Keys and defaults verified against the Patroni 3.3 documentation:
//   - https://patroni.readthedocs.io/en/latest/yaml_configuration.html
//   - https://patroni.readthedocs.io/en/latest/dynamic_configuration.html
//
// Two Patroni 3.x specifics are modeled explicitly: `synchronous_mode` is a
// three-valued setting (off / on / quorum, the quorum mode being the 3.x
// addition), and `failsafe_mode` — the DCS failsafe introduced in 3.0 — is a
// first-class key.
//
// PostgreSQL parameters are deliberately NOT here: they live in
// cfg.postgresql.conf@<major>, and the server injects the rendered map under
// bootstrap.dcs.postgresql.parameters when it bakes the RunSpec. The flat
// `postgresql_parameters` map below is the escape hatch for the handful of
// keys Patroni itself must own (they are written verbatim, in no fixed order).
//
//nolint:funlen // one flat key table
func PatroniYml3() *schemapb.Schema { return patroniYml(3) }

// PatroniYml4 covers the shared Patroni 4.1 configuration keys.
// doc: https://patroni.readthedocs.io/en/latest/yaml_configuration.html
func PatroniYml4() *schemapb.Schema { return patroniYml(4) }

//nolint:funlen // one flat key table
func patroniYml(major uint64) *schemapb.Schema {
	template := strings.Replace(patroniTemplate, "Patroni 3.x", fmt.Sprintf("Patroni %d.x", major), 1)
	if major >= 4 {
		// pg_hba is an inline list; an external file is a PostgreSQL hba_file parameter.
		template = strings.Replace(template, "  pg_hba: {{{.}}}", "  parameters:\n    hba_file: {{{.}}}", 1)
	}
	return schemapb.NewSchema(ids.Cfg("patroni.yml", major)).
		Descr(fmt.Sprintf("patroni.yml for Patroni %d.x with an etcd3 DCS.", major)).
		Strict().Coerce().
		Fields(
			// ------------------------------------------------------- identity
			// doc: yaml_configuration.html — scope / name
			schemapb.Str("scope").Title("Cluster scope").Group("Cluster").
				Desc("Cluster name; every member of one Patroni cluster shares it. Filled by the server from the run's topology.").
				Pattern(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`).Default("stroppy"),
			schemapb.Str("name").Title("Node name").Group("Cluster").
				Desc("Unique member name inside the scope. Filled by the server from the node's role index.").
				Pattern(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`).Nullable(),
			schemapb.Str("dcs_namespace").Title("DCS namespace").Group("Cluster").
				// doc: yaml_configuration.html — namespace, default /service/
				Desc("Key prefix Patroni uses inside the DCS; rendered as the top-level `namespace` key (the field cannot be named `namespace`: it is a CEL reserved word).").
				Pattern(`^/.*$`).Default("/service/"),

			// -------------------------------------------------------- restapi
			// doc: yaml_configuration.html — restapi
			schemapb.Str("restapi_listen").Title("REST API listen").Group("REST API").
				Desc("host:port the Patroni REST API binds to; rendered as restapi.listen.").
				Pattern(`^[^\s]+:[0-9]{1,5}$`).Default("0.0.0.0:8008"),
			schemapb.Str("restapi_connect_address").Title("REST API connect address").Group("Cluster").
				Desc("host:port other members and haproxy use to reach this node's REST API. Filled by the server from the node's address.").
				Pattern(`^[^\s]+:[0-9]{1,5}$`).Nullable(),

			// ---------------------------------------------------------- etcd3
			// doc: yaml_configuration.html — etcd3.hosts
			schemapb.List("etcd3_hosts", schemapb.Str("host").Pattern(`^[^\s]+:[0-9]{1,5}$`)).
				Title("etcd3 hosts").Group("Cluster").
				Desc("host:port list of the etcd v3 cluster backing the DCS. Filled by the server from the etcd topology.").
				MaxItems(16).Unique().Nullable(),
			schemapb.Computed("etcd3_hosts_rendered",
				`("etcd3_hosts" in root) ? root.etcd3_hosts.map(h, "      - " + h).join("\n") : ""`).
				Title("Rendered etcd3 hosts").Group("Cluster").
				Desc("The etcd3 host list as an indented YAML sequence (the render context does not expand lists).").
				Result(schemapb.ResultString),

			// ------------------------------------------------- bootstrap.dcs
			schemapb.Int64("ttl").Title("Leader TTL").Group("DCS").Unit("s").
				// doc: dynamic_configuration.html — ttl, default 30
				Desc("Lifetime of the leader lock: how long a leaderless cluster waits before failover.").
				Gte(20).Lte(3600).Default(30),
			schemapb.Int64("loop_wait").Title("Loop wait").Group("DCS").Unit("s").
				// doc: dynamic_configuration.html — loop_wait, default 10
				Desc("Sleep between Patroni's heartbeat cycles.").
				Gte(1).Lte(600).Default(10),
			schemapb.Int64("retry_timeout").Title("Retry timeout").Group("DCS").Unit("s").
				// doc: dynamic_configuration.html — retry_timeout, default 10
				Desc("Timeout for DCS and PostgreSQL operations before they are retried.").
				Gte(3).Lte(600).Default(10),
			schemapb.Int64("maximum_lag_on_failover").Title("Maximum lag on failover").Group("DCS").Unit("B").
				// doc: dynamic_configuration.html — maximum_lag_on_failover, default 1048576
				Desc("Replication lag, in bytes, above which a replica may not be promoted.").
				Gte(0).Lte(1099511627776).Default(1048576),
			schemapb.Choice("synchronous_mode").Title("Synchronous mode").Group("DCS").
				// doc: dynamic_configuration.html — synchronous_mode: off | on | quorum
				Desc("Synchronous replication mode. `quorum` (quorum-based commit) is the Patroni 3.x addition.").
				Opt(schemapb.StrV("off"), "off").Opt(schemapb.StrV("on"), "on").
				Opt(schemapb.StrV("quorum"), "quorum").
				Default(schemapb.StrV("off")),
			trueFalse("synchronous_mode_strict", "false").Title("Synchronous mode strict").Group("DCS").
				// doc: dynamic_configuration.html — synchronous_mode_strict, default false
				Desc("Never drop back to asynchronous replication, even with no healthy synchronous standby: writes stall instead."),
			schemapb.Int64("synchronous_node_count").Title("Synchronous node count").Group("DCS").
				// doc: dynamic_configuration.html — synchronous_node_count, default 1
				Desc("Number of synchronous standbys Patroni maintains when synchronous_mode is on.").
				Gte(1).Lte(16).Default(1),
			trueFalse("failsafe_mode", "false").Title("Failsafe mode").Group("DCS").
				// doc: dynamic_configuration.html — failsafe_mode, default false; added in Patroni 3.0
				Desc("Keep the primary running when the DCS is unreachable but every member answers the failsafe endpoint. Patroni 3.0+."),
			trueFalse("use_pg_rewind", "true").Title("Use pg_rewind").Group("DCS").
				// doc: dynamic_configuration.html — postgresql.use_pg_rewind, upstream default false
				Desc("Rejoin a demoted primary with pg_rewind instead of a full basebackup. Product default true: a stroppy rerun must not spend a basebackup on every failover."),
			trueFalse("use_slots", "true").Title("Use replication slots").Group("DCS").
				// doc: dynamic_configuration.html — postgresql.use_slots, default true
				Desc("Have Patroni manage physical replication slots for the members."),

			// ------------------------------------------------------ postgresql
			// doc: yaml_configuration.html — postgresql.*
			schemapb.Str("postgresql_listen").Title("PostgreSQL listen").Group("PostgreSQL").
				Desc("host:port PostgreSQL binds to; rendered as postgresql.listen.").
				Pattern(`^[^\s]+:[0-9]{1,5}$`).Default("0.0.0.0:5432"),
			schemapb.Str("postgresql_connect_address").Title("PostgreSQL connect address").Group("Cluster").
				Desc("host:port other members use to reach this PostgreSQL. Filled by the server from the node's address.").
				Pattern(`^[^\s]+:[0-9]{1,5}$`).Nullable(),
			schemapb.Str("data_dir").Title("Data directory").Group("PostgreSQL").
				// doc: yaml_configuration.html — postgresql.data_dir
				Desc("PGDATA directory Patroni initializes and manages.").
				Pattern(`^/.+$`).Default("/var/lib/postgresql/data"),
			schemapb.Str("bin_dir").Title("Binary directory").Group("PostgreSQL").
				// doc: yaml_configuration.html — postgresql.bin_dir, default '' (PATH)
				Desc("Directory holding the PostgreSQL binaries; empty means look them up on PATH.").
				Pattern(`^/.+$`).Default("/usr/lib/postgresql/17/bin"),
			schemapb.Str("pg_hba_path").Title("pg_hba.conf path").Group("PostgreSQL").
				// doc: yaml_configuration.html — postgresql.pg_hba
				Desc("Path of the pg_hba.conf rendered from cfg.pg_hba.conf@1. Patroni's inline `pg_hba` list is not used: the file is the single source.").
				Pattern(`^/.+$`).Nullable(),

			// -------------------------------------------------- authentication
			// doc: yaml_configuration.html — postgresql.authentication
			schemapb.Str("superuser_username").Title("Superuser").Group("Authentication").
				Desc("Superuser role Patroni connects with.").
				MinLen(1).MaxLen(63).Default("postgres"),
			schemapb.Str("superuser_password").Title("Superuser password").Group("Authentication").
				Desc("Password of the superuser role.").
				MinLen(1).MaxLen(256).Secret().Nullable(),
			schemapb.Str("replication_username").Title("Replication user").Group("Authentication").
				Desc("Role used for streaming replication between members.").
				MinLen(1).MaxLen(63).Default("replicator"),
			schemapb.Str("replication_password").Title("Replication password").Group("Authentication").
				Desc("Password of the replication role.").
				MinLen(1).MaxLen(256).Secret().Nullable(),
			schemapb.Str("rewind_username").Title("Rewind user").Group("Authentication").
				Desc("Role pg_rewind runs as; only needed when use_pg_rewind is true and the superuser is not used.").
				MinLen(1).MaxLen(63).Default("rewind"),
			schemapb.Str("rewind_password").Title("Rewind password").Group("Authentication").
				Desc("Password of the rewind role.").
				MinLen(1).MaxLen(256).Secret().Nullable(),

			// ----------------------------------------------------------- tags
			// doc: yaml_configuration.html — tags
			trueFalse("tag_nofailover", "false").Title("nofailover").Group("Tags").
				Desc("Exclude this member from leader races."),
			trueFalse("tag_noloadbalance", "false").Title("noloadbalance").Group("Tags").
				Desc("Make the /replica REST endpoint return 503 so load balancers skip this member."),
			trueFalse("tag_clonefrom", "false").Title("clonefrom").Group("Tags").
				Desc("Prefer this member as the source of a basebackup for new replicas."),
			trueFalse("tag_nosync", "false").Title("nosync").Group("Tags").
				Desc("Never choose this member as a synchronous standby."),

			// ------------------------------------------------------- watchdog
			// doc: yaml_configuration.html — watchdog
			schemapb.Choice("watchdog_mode").Title("Watchdog mode").Group("Watchdog").
				Desc("Watchdog use: `automatic` uses it when available, `required` refuses to promote without it, `off` disables it.").
				Opt(schemapb.StrV("off"), "off").Opt(schemapb.StrV("automatic"), "automatic").
				Opt(schemapb.StrV("required"), "required").
				Default(schemapb.StrV("automatic")),
			schemapb.Str("watchdog_device").Title("Watchdog device").Group("Watchdog").
				// doc: yaml_configuration.html — watchdog.device, default /dev/watchdog
				Desc("Watchdog device node.").
				Pattern(`^/.+$`).Default("/dev/watchdog"),
			schemapb.Int64("watchdog_safety_margin").Title("Watchdog safety margin").Group("Watchdog").Unit("s").
				// doc: yaml_configuration.html — watchdog.safety_margin, default 5
				Desc("Seconds the watchdog must fire before the leader key expires; -1 keeps a fixed half-TTL margin.").
				Gte(-1).Lte(600).Default(5),

			// ------------------------------------------- postgresql parameters
			schemapb.MapOf("postgresql_parameters", schemapb.Str("value")).
				Title("Patroni-owned PostgreSQL parameters").Group("Custom").
				Desc("PostgreSQL parameters Patroni itself must manage in the DCS (the rest of postgresql.conf comes from cfg.postgresql.conf@<major>). Entry order is not preserved."),
			schemapb.Computed("postgresql_parameters_rendered",
				`("postgresql_parameters" in root)`+
					` ? root.postgresql_parameters.map(k, "        " + k + ": \"" + string(root.postgresql_parameters[k]) + "\"").join("\n")`+
					` : ""`).
				Title("Rendered parameters").Group("Custom").
				Desc("The postgresql_parameters map as an indented YAML mapping.").
				Result(schemapb.ResultString),
		).
		Rules(
			schemapb.Rule(`int(root.ttl) >= int(root.loop_wait) + int(root.retry_timeout) * 2`,
				"ttl must be at least loop_wait + 2 * retry_timeout, otherwise the leader key expires mid-cycle").
				ID("ttl-vs-loop"),
			schemapb.Rule(`root.synchronous_mode != "off" || root.synchronous_mode_strict == "false"`,
				"synchronous_mode_strict requires synchronous_mode to be on or quorum").ID("strict-needs-sync"),
			schemapb.Rule(`root.watchdog_mode == "off" || int(root.watchdog_safety_margin) < int(root.ttl)`,
				"watchdog safety_margin must be below ttl").ID("watchdog-margin"),
		).
		Template("conf", template).
		MustBuild()
}

const patroniTemplate = `# patroni.yml — generated by stroppy for Patroni 3.x.
scope: {{{values.scope}}}
namespace: {{{values.dcs_namespace}}}
{{#values.name}}name: {{{.}}}
{{/values.name}}
restapi:
  listen: {{{values.restapi_listen}}}
{{#values.restapi_connect_address}}  connect_address: {{{.}}}
{{/values.restapi_connect_address}}
{{#values.etcd3_hosts_rendered}}etcd3:
  hosts:
{{{values.etcd3_hosts_rendered}}}
{{/values.etcd3_hosts_rendered}}
bootstrap:
  dcs:
    ttl: {{{values.ttl}}}
    loop_wait: {{{values.loop_wait}}}
    retry_timeout: {{{values.retry_timeout}}}
    maximum_lag_on_failover: {{{values.maximum_lag_on_failover}}}
    synchronous_mode: {{{values.synchronous_mode}}}
    synchronous_mode_strict: {{{values.synchronous_mode_strict}}}
    synchronous_node_count: {{{values.synchronous_node_count}}}
    failsafe_mode: {{{values.failsafe_mode}}}
    postgresql:
      use_pg_rewind: {{{values.use_pg_rewind}}}
      use_slots: {{{values.use_slots}}}
{{#values.postgresql_parameters_rendered}}      parameters:
{{{values.postgresql_parameters_rendered}}}
{{/values.postgresql_parameters_rendered}}
postgresql:
  listen: {{{values.postgresql_listen}}}
{{#values.postgresql_connect_address}}  connect_address: {{{.}}}
{{/values.postgresql_connect_address}}  data_dir: {{{values.data_dir}}}
  bin_dir: {{{values.bin_dir}}}
{{#values.pg_hba_path}}  pg_hba: {{{.}}}
{{/values.pg_hba_path}}  authentication:
    superuser:
      username: {{{values.superuser_username}}}
{{#values.superuser_password}}      password: "{{{.}}}"
{{/values.superuser_password}}    replication:
      username: {{{values.replication_username}}}
{{#values.replication_password}}      password: "{{{.}}}"
{{/values.replication_password}}    rewind:
      username: {{{values.rewind_username}}}
{{#values.rewind_password}}      password: "{{{.}}}"
{{/values.rewind_password}}
tags:
  nofailover: {{{values.tag_nofailover}}}
  noloadbalance: {{{values.tag_noloadbalance}}}
  clonefrom: {{{values.tag_clonefrom}}}
  nosync: {{{values.tag_nosync}}}

watchdog:
  mode: {{{values.watchdog_mode}}}
  device: {{{values.watchdog_device}}}
  safety_margin: {{{values.watchdog_safety_margin}}}
`
