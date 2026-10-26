package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// PgNoop is db.pg_noop.params@1 — a blackhole that speaks the PostgreSQL wire
// protocol and answers everything without storing anything. It measures the
// ceiling of delivery: driver, network and protocol handling, with the storage
// engine taken out of the picture. Its counterpart db.noop.params removes the
// network too.
//
// doc: https://www.postgresql.org/docs/current/protocol.html
func PgNoop() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("pg_noop", 1)).
		Descr("PostgreSQL wire-protocol blackhole; measures the delivery ceiling with no storage engine behind it.").
		Strict().Coerce().
		Fields(
			schemapb.Choice("version").Title("pg-noop version").Group("Engine").
				Desc("Build of the pg-noop server; the binary is served through the stroppy gateway.").
				Opt(schemapb.StrV("0.2.0"), "0.2.0").
				Opt(schemapb.StrV("0.1.2"), "0.1.2").
				Default(schemapb.StrV("0.2.0")).Required(),

			schemapb.Int64("workers").Title("Workers").Group("Server").
				Desc("Connection-handling workers; 0 = one per CPU of the machine.").
				Gte(0).Lte(4096).Default(0),

			// doc: https://www.postgresql.org/docs/current/protocol-flow.html
			schemapb.Int64("latency_ms").Title("Injected latency").Group("Behavior").
				Desc("Artificial delay added before each response, to model a storage engine that is not free.").
				Unit("ms").Gte(0).Lte(60000).Default(0),

			// doc: https://www.postgresql.org/docs/current/protocol-error-fields.html
			schemapb.Double("error_rate").Title("Error rate").Group("Behavior").
				Desc("Fraction of statements answered with an ErrorResponse instead of a result, to exercise the driver's error path. 0 = never, 1 = always.").
				Gte(0).Lte(1).Default(0),

			schemapb.Int64("port").Title("Port").Group("Server").
				Desc("TCP port the blackhole listens on.").
				Gte(1).Lte(65535).Default(5432),
		).
		Rules(
			schemapb.Rule(`root.error_rate == 0.0 || root.latency_ms == 0`,
				"injecting both latency and errors makes the ceiling unattributable").ID("one-knob-at-a-time").Warn(),
		).
		MustBuild()
}
