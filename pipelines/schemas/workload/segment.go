// Package workload holds the schemas of a stroppy workload: one load segment
// and the whole workload record (version, protocol, segments, driver options).
//
// Everything here follows stroppy v6 — the Go-native engine without k6. The
// authority is the stroppy checkout itself (`stroppy probe -o json`,
// `stroppy run <workload> --help`, `stroppy help config-file`); every knob
// below names the stroppy parameter it maps to. Builds older than 6.0.0 are
// not supported and have no fallbacks.
package workload

import (
	"strings"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// stepPattern is a stroppy step id (drop_schema, create_schema, load_data,
// workload_tx_new_order, ...).
// doc: stroppy `help steps`
const stepPattern = `^[a-z][a-z0-9_]*$`

// flagPattern is a typed stroppy parameter flag without the leading dashes
// (`--load-workers` → load-workers).
// doc: stroppy `run <workload> --help` — "--name VALUE"
const flagPattern = `^[a-z][a-z0-9-]*$`

// filePattern is a stroppy preset dialect id (tpcc/pico).
const filePattern = `^[A-Za-z0-9._/-]+$`

// Scripts stroppy 6.0.0 registers (`stroppy probe -o json` → workloads[].name).
// The catalog (system.stroppy_catalog@1) may add more for newer builds; those
// pass their parameters through extra_params.
const (
	ScriptTpccTx     schemapb.VariantKey = "tpcc/tx"
	ScriptTpccProcs  schemapb.VariantKey = "tpcc/procs"
	ScriptTpcbTx     schemapb.VariantKey = "tpcb/tx"
	ScriptTpcbProcs  schemapb.VariantKey = "tpcb/procs"
	ScriptTpchTx     schemapb.VariantKey = "tpch/tx"
	ScriptTpcds      schemapb.VariantKey = "tpcds"
	ScriptSimple     schemapb.VariantKey = "simple"
	ScriptBaseline   schemapb.VariantKey = "baseline"
	ScriptExecuteSQL schemapb.VariantKey = "execute_sql"
)

// Executors stroppy 6 knows (pkg/bench/runtime.go).
const (
	ExecutorConstantVUs      = "constant-vus"
	ExecutorSharedIterations = "shared-iterations"
)

// strictVariant builds one oneof variant that rejects unknown keys — a
// parameter of another script (or a typo) must not pass silently. The
// discriminator is declared so strict mode knows it.
func strictVariant(fields ...schemapb.FieldDef) *schemapb.Schema {
	s := &schemapb.Schema{Strict: true}
	s.Fields = append(s.Fields, schemapb.Str("script").Required().Done())
	for _, f := range fields {
		s.Fields = append(s.Fields, f.Done())
	}
	return s
}

// --- shared workload parameters ---------------------------------------------
// Names are the stroppy config keys in snake_case; the pipeline converts them
// to the lowerCamel `params` object of stroppy-config.json.

// doc: probe — loadWorkers int "Workers used to load each table."
func loadWorkers(def int64) *schemapb.NumB[int64] {
	return schemapb.Int64("load_workers").Title("Load workers").Group("Load").
		Desc("Workers used to load each table (loadWorkers).").
		Gte(0).Lte(1024).Default(def)
}

// doc: probe — retryAttempts int default 3
func retryAttempts() *schemapb.NumB[int64] {
	return schemapb.Int64("retry_attempts").Title("Retry attempts").Group("Transactions").
		Desc("Maximum attempts of one transaction before the iteration fails (retryAttempts).").
		Gte(1).Lte(100).Default(3)
}

// doc: stroppy AGENTS.md "Full isolation type names"; empty = driver default
// (postgres/mysql read_committed, picodata none, ydb serializable).
func txIsolation() *schemapb.ChoiceB {
	return schemapb.Choice("tx_isolation").Title("Transaction isolation").Group("Transactions").
		Desc("Isolation override (txIsolation); unset keeps the driver default. Picodata only supports none.").
		Opt(schemapb.StrV("read_uncommitted"), "Read uncommitted").
		Opt(schemapb.StrV("read_committed"), "Read committed").
		Opt(schemapb.StrV("repeatable_read"), "Repeatable read").
		Opt(schemapb.StrV("serializable"), "Serializable").
		Opt(schemapb.StrV("db_default"), "Database default").
		Opt(schemapb.StrV("conn"), "Connection-level").
		Opt(schemapb.StrV("none"), "None (no BEGIN)").
		Nullable()
}

// doc: probe — pgUnlogged bool default false
func pgUnlogged() *schemapb.BoolB {
	return schemapb.Bool("pg_unlogged").Title("Unlogged tables while loading").Group("Load").
		Desc("Use unlogged PostgreSQL tables during the load, then set them logged (pgUnlogged). PostgreSQL only.").
		Default(false)
}

// doc: probe — sqlFile string "SQL dialect file override."
func sqlFile(desc string) *schemapb.StrB {
	return schemapb.Str("sql_file").Title("SQL file").Group("SQL").
		Desc(desc).Pattern(filePattern).MaxLen(256).Nullable()
}

// doc: probe — ydbStoreMode string default "column"
func ydbStoreMode() *schemapb.ChoiceB {
	return schemapb.Choice("ydb_store_mode").Title("YDB store mode").Group("Load").
		Desc("YDB table store mode (ydbStoreMode); ignored by other drivers.").
		Opt(schemapb.StrV("column"), "Column store").
		Opt(schemapb.StrV("row"), "Row store").
		Default(schemapb.StrV("column"))
}

// tpccFields are the parameters of tpcc/tx and tpcc/procs.
// doc: probe — tpcc/tx params; AGENTS.md "--scale-factor semantics" (integer).
func tpccFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		schemapb.Int64("scale_factor").Title("Warehouses").Group("Data").
			Desc("Number of warehouses (scaleFactor); ~100 MB per warehouse drives the disk requirement.").
			Gte(1).Lte(100000).Default(1),
		schemapb.Int64("warehouse_start").Title("First warehouse").Group("Data").
			Desc("First warehouse id (warehouseStart); lets several runners share one database.").
			Gte(1).Lte(100000).Default(1),
		schemapb.Bool("load_items").Title("Load items").Group("Load").
			Desc("Load the shared item table (loadItems); unset = only when warehouse_start is 1.").
			Nullable(),
		loadWorkers(1),
		pgUnlogged(),
		schemapb.Bool("pacing").Title("Pacing").Group("Transactions").
			Desc("Apply TPC-C keying and think times (pacing); needed for a compliance verdict.").
			Default(false),
		retryAttempts(),
		txIsolation(),
		sqlFile("Dialect file override (sqlFile): a preset id shipped with stroppy, like tpcc/ydb_no_indexes."),
	}
}

// tpcbFields are the parameters of tpcb/tx and tpcb/procs.
// doc: probe — tpcb/tx params.
func tpcbFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		schemapb.Int64("scale_factor").Title("Scale factor").Group("Data").
			Desc("TPC-B scale factor = branches (scaleFactor); 100k accounts per branch.").
			Gte(1).Lte(100000).Default(1),
		loadWorkers(1),
		retryAttempts(),
		txIsolation(),
		sqlFile("Dialect file override (sqlFile): a preset id shipped with stroppy, like tpcb/pico."),
	}
}

// tpchFields are the parameters of tpch/tx.
// doc: probe — tpch/tx params; AGENTS.md: fractional SF allowed (0.01).
func tpchFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		schemapb.Double("scale_factor").Title("Scale factor").Group("Data").
			Desc("TPC-H scale factor (scaleFactor); fractional allowed, 1 ≈ 1 GB of data.").
			Gt(0).Lte(10000).Default(1),
		loadWorkers(0).Desc("Workers used to load each table (loadWorkers); 0 = automatic."),
		pgUnlogged(),
		ydbStoreMode(),
		sqlFile("Dialect file override (sqlFile): a preset id shipped with stroppy, like tpch/pico."),
	}
}

// tpcdsFields are the parameters of tpcds.
// doc: probe — tpcds params.
func tpcdsFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		schemapb.Double("scale_factor").Title("Scale factor").Group("Data").
			Desc("TPC-DS scale factor (scaleFactor); fractional allowed. Static dimensions (~1.9M customer_demographics rows) do not shrink.").
			Gt(0).Lte(10000).Default(1),
		loadWorkers(0).Desc("Workers used to load each table (loadWorkers); 0 = automatic."),
		pgUnlogged(),
		schemapb.Int64("streams").Title("Query streams").Group("Queries").
			Desc("Number of query streams (streams).").Gte(1).Lte(64).Default(1),
		schemapb.Int64("query_stream").Title("Query stream").Group("Queries").
			Desc("Generated query stream (queryStream); unset uses the baked query set. Explicit zero selects generated stream 0.").Gte(0).Nullable(),
		schemapb.Int64("query_seed").Title("Query seed").Group("Queries").
			Desc("Query generator seed (querySeed).").Default(19620718),
		schemapb.Bool("validate_force").Title("Validate outside SF=1").Group("Queries").
			Desc("Compare answers even when the scale factor is not 1 (validateForce).").Default(false),
		ydbStoreMode(),
		schemapb.Str("schema_file").Title("Schema file").Group("SQL").
			Desc("Schema SQL override (schemaFile): a preset id shipped with stroppy, like tpcds/schema.pico.").
			Pattern(filePattern).MaxLen(256).Nullable(),
		sqlFile("Query SQL override (sqlFile): a preset id shipped with stroppy, like tpcds/pico."),
	}
}

// baselineFields describes `stroppy run baseline`, independently of the
// machine self-check command `stroppy baseline`.
func baselineFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		loadWorkers(20),
		schemapb.Int64("rows").Title("Rows").Group("Data").Desc("Rows loaded into the baseline probe table (rows).").Gte(1).Default(250000),
		txIsolation(),
	}
}

// executeSQLFields are the parameters of execute_sql.
// doc: probe — execute_sql params: sqlBody | sqlFile.
func executeSQLFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		schemapb.Str("sql_body").Title("Inline SQL").Group("SQL").
			Desc("SQL text to execute (sqlBody); may start with a `--= name` marker to name the query.").
			MaxLen(1 << 20).Nullable(),
		sqlFile("SQL file to execute (sqlFile): a preset id shipped with stroppy; use sql_body for inline SQL."),
	}
}

// Segment is workload.segment@1 — one `stroppy run` invocation: which
// workload with which typed parameters, the scenario (executor/VUs/bound),
// step filters and what the pipeline checks afterwards.
//
// doc: stroppy `help config-file` — script, run{executor,vus,iterations,
// duration,queryTimeout}, params{...}, steps/noSteps.
func Segment() *schemapb.Schema {
	return schemapb.NewSchema(ids.Workload("segment", 1)).
		Descr("One stroppy load segment: workload, typed parameters, scenario, steps and thresholds.").
		Strict().Coerce().
		Fields(
			schemapb.Str("name").Title("Name").Group("Segment").
				Desc("Segment id inside the workload; used as the phase label of the run and as a metric label.").
				Pattern(`^[a-z][a-z0-9_-]*$`).MinLen(1).MaxLen(64).Required(),

			// doc: `stroppy probe -o json` → workloads[].name; the discriminator
			// IS the stroppy script id passed to `stroppy run`.
			schemapb.OneOf("workload", "script").Title("Workload").Group("Workload").
				Desc("Built-in stroppy workload and its typed parameters; `script` is the id passed to `stroppy run`.").
				VariantOf(ScriptTpccTx, strictVariant(tpccFields()...)).
				VariantOf(ScriptTpccProcs, strictVariant(tpccFields()...)).
				VariantOf(ScriptTpcbTx, strictVariant(tpcbFields()...)).
				VariantOf(ScriptTpcbProcs, strictVariant(tpcbFields()...)).
				VariantOf(ScriptTpchTx, strictVariant(tpchFields()...)).
				VariantOf(ScriptTpcds, strictVariant(tpcdsFields()...)).
				VariantOf(ScriptSimple, strictVariant()).
				VariantOf(ScriptBaseline, strictVariant(baselineFields()...)).
				VariantOf(ScriptExecuteSQL, strictVariant(executeSQLFields()...)).
				Required(),

			// doc: `stroppy run <workload> --help` "Run parameters"
			schemapb.Object("run",
				schemapb.Choice("executor").Title("Executor").
					Desc("Scenario executor: constant-vus runs for a duration, shared-iterations shares N iterations between VUs.").
					Opt(schemapb.StrV(ExecutorConstantVUs), "Constant VUs for a duration").
					Opt(schemapb.StrV(ExecutorSharedIterations), "Shared iterations").
					Default(schemapb.StrV(ExecutorConstantVUs)),
				schemapb.Int64("vus").Title("Virtual users").
					Desc("Concurrent virtual users (vus); sizes the runner machine and the pool.").
					Gte(1).Lte(100000).Default(1),
				schemapb.Duration("duration").Title("Duration").
					Desc("Wall-clock length of a constant-vus scenario (duration).").
					Gt(0).Lte(24*time.Hour).Nullable(),
				schemapb.Int64("iterations").Title("Iterations").
					Desc("Total iterations of a shared-iterations scenario (iterations).").
					Gte(1).Nullable(),
				schemapb.Duration("query_timeout").Title("Query timeout").
					Desc("Per-statement deadline (queryTimeout); 0 disables it.").
					Gte(0).Lte(time.Hour).Default(0),
			).Title("Scenario").Group("Scenario").
				Desc("Load shape of the segment.").Strict().Required().
				Rule(
					schemapb.Rule(
						`this.executor != "`+ExecutorConstantVUs+`" || ("duration" in this && this.duration != null)`,
						"constant-vus needs a duration",
					).ID("constant-vus-needs-duration"),
					schemapb.Rule(`this.executor != "constant-vus" || !("iterations" in this) || this.iterations == null`, "iterations applies only to shared-iterations").ID("iterations-executor"),
					schemapb.Rule(`this.executor != "shared-iterations" || !("duration" in this) || this.duration == null`, "duration applies only to constant-vus").ID("duration-executor"),
					schemapb.Rule(
						`this.executor != "`+ExecutorSharedIterations+`" || ("iterations" in this && this.iterations != null)`,
						"shared-iterations needs an iteration count",
					).ID("shared-iterations-needs-count"),
				),

			// doc: stroppy `help steps`
			schemapb.List("steps",
				schemapb.Str("").Pattern(stepPattern).MinLen(1).MaxLen(64),
			).Title("Only these steps").Group("Steps").
				Desc("Run only the listed stroppy steps (--steps); empty = all steps.").
				MaxItems(32).Unique(),
			schemapb.List("no_steps",
				schemapb.Str("").Pattern(stepPattern).MinLen(1).MaxLen(64),
			).Title("Skip these steps").Group("Steps").
				Desc("Skip the listed stroppy steps (--no-steps); stroppy rejects it together with steps.").
				MaxItems(32).Unique(),

			// Escape hatch for parameters a newer stroppy build declares and
			// this schema does not know yet: rendered as `--<flag> <value>`.
			schemapb.MapOf("extra_params", schemapb.Str("value").MaxLen(4096)).
				Title("Extra parameters").Group("Workload").
				Desc("Typed stroppy flags this form does not model, by flag name without dashes (load-workers); values are parsed by stroppy.").
				MaxEntries(64).
				Rules(schemapb.Rule(
					`this.all(k, k.matches("`+flagPattern+`"))`,
					"extra parameter keys must be stroppy flag names (lower-kebab)",
				).ID("extra-param-key-shape")),

			// Evaluated by the pipeline from the bench summary — stroppy 6 has
			// no threshold engine of its own.
			schemapb.Object("thresholds",
				schemapb.Double("p99_ms").Title("p99 latency").Unit("ms").
					Desc("Fail the segment when iteration_duration p99 exceeds this.").Gt(0),
				schemapb.Double("error_rate").Title("Error rate").Unit("ratio").
					Desc("Fail the segment when failed_iterations / iterations exceeds this (0..1).").Gte(0).Lte(1),
			).Title("Thresholds").Group("Thresholds").
				Desc("Pass/fail bounds the pipeline applies to the segment summary.").Strict(),

			schemapb.UInt64("seed").Title("Random seed").Group("Scenario").
				Desc("Stroppy global.seed; zero uses Stroppy's random seed, a positive value makes generation reproducible.").Gte(0).Nullable(),
			schemapb.Duration("timeout").Title("Segment timeout").Group("Scenario").
				Desc("Execution deadline after the warmup wait, including container preparation and data load; unset uses duration plus headroom, or 24h for iterations.").Gt(0).Lte(7*24*time.Hour).Nullable(),
			schemapb.Duration("warmup").Title("Warm-up").Group("Scenario").
				Desc("Idle wait before the segment starts, letting caches and replicas settle.").
				Gte(0).Lte(time.Hour).Default(0),

			// doc: `stroppy run --help` Logging — --log-level
			schemapb.Choice("log_level").Title("Log level").Group("Segment").
				Desc("Minimum stroppy log level (--log-level); debug traces parameter resolution.").
				Opt(schemapb.StrV("debug"), "debug").
				Opt(schemapb.StrV("info"), "info").
				Opt(schemapb.StrV("warn"), "warn").
				Opt(schemapb.StrV("error"), "error").
				Default(schemapb.StrV("info")),
		).
		Rules(
			segmentRule(stepFilterRule(), "step must be an executable Stroppy step of the selected workload").ID("known-workload-steps"),
			segmentRule(
				`!("steps" in this) || !("no_steps" in this) || size(this.steps) == 0 || size(this.no_steps) == 0`,
				"stroppy rejects --steps together with --no-steps",
			).ID("steps-mutually-exclusive"),
			segmentRule(
				`!("workload" in this) || this.workload.script != "`+string(ScriptExecuteSQL)+`" || `+
					`(("sql_body" in this.workload && this.workload.sql_body != null) != ("sql_file" in this.workload && this.workload.sql_file != null))`,
				"execute_sql needs exactly one of sql_body or sql_file",
			).ID("execute-sql-source"),
		).
		MustBuild()
}

// Segment is used both as a root schema and as a nested object. CEL root
// remains the outer document; this is nil only at the root.
func segmentRule(expr, message string) *schemapb.RuleB {
	return schemapb.Rule("[this == null ? root : this].all(s, "+strings.ReplaceAll(expr, "this", "s")+")", message)
}
