package spec

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// metricValue is the canonical metric shape shared by the run result and each
// segment. It matches OpenAPI MetricValue
// (openapi/parts/61-components-runs.yaml).
func metricValue(name schemapb.FieldName) *schemapb.MapB {
	return schemapb.Map(name,
		schemapb.Double("value").Title("Value").
			Desc("Headline value of the metric over the window.").Required(),
		schemapb.Str("unit").Title("Unit").
			Desc("Unit of the value: tps, ms, ratio…").MaxLen(32),
		schemapb.Double("min").Title("Minimum").Desc("Lowest sample in the window."),
		schemapb.Double("max").Title("Maximum").Desc("Highest sample in the window."),
		schemapb.Double("avg").Title("Average").Desc("Mean over the window."),
	).Strict().MaxEntries(256)
}

// ResultRun is spec.result.run@1 — the RunResult the stroppy-run pipeline
// returns. It is the only run data that travels back through Graphene; raw
// stroppy output stays an artifact and the time series stay in Victoria.
//
// The JSON shape matches OpenAPI RunResult
// (openapi/parts/61-components-runs.yaml).
//
// doc: STROPPY.MD §6.1 step 5, §16.6 "Данные результата".
func ResultRun() *schemapb.Schema {
	return schemapb.NewSchema(ids.Spec("result.run", 1)).
		Descr("RunResult: headline metrics, per-segment outcome, artifacts and summary of one run.").
		Strict().Coerce().
		Fields(
			metricValue("metrics").Title("Metrics").Group("Result").
				Desc("Metrics keyed by segment and metric name; up to 256 metrics for each of 64 segments.").
				MaxEntries(64*256),

			schemapb.List("segments",
				schemapb.Object("",
					schemapb.Str("name").Title("Segment").
						Desc("Name of the workload segment.").
						Pattern(`^[a-z][a-z0-9_-]*$`).MaxLen(64).Required(),
					schemapb.Choice("status").Title("Status").
						Desc("How the segment ended.").
						Opt(schemapb.StrV("completed"), "Completed").
						Opt(schemapb.StrV("failed"), "Failed").
						Opt(schemapb.StrV("skipped"), "Skipped").
						Opt(schemapb.StrV("canceled"), "Canceled").
						Required(),
					schemapb.Timestamp("started_at").Title("Started"),
					schemapb.Timestamp("finished_at").Title("Finished"),
					metricValue("metrics").Title("Metrics").
						Desc("Metrics measured over this segment's window only (bench summary counters and histogram statistics)."),
					// doc: stroppy `help drivers` ERROR AND EXIT BEHAVIOR — the
					// "completed with errors" block of the summary.
					schemapb.Object("errors",
						schemapb.Int64("terminal_errors").Title("Terminal errors").Gte(0),
						schemapb.Int64("failed_iterations").Title("Failed iterations").Gte(0),
						schemapb.Int64("failed_queries").Title("Failed queries").Gte(0),
						schemapb.Int64("retry_attempts").Title("Retry attempts").Gte(0),
					).Title("Nonfatal errors").
						Desc("Stroppy's own error accounting; nonfatal errors keep the exit status 0.").Strict(),
					schemapb.Int64("exit_code").Title("Exit code").
						Desc("Exit status of the stroppy process (0 ok, 130/143 canceled, 1 error).").Gte(-1).Lte(255),
					// doc: stroppy workloads/tpcc/report.go — one JSON line
					// {"compliance": Report} on stdout.
					schemapb.JSON("compliance").Title("TPC-C compliance report").
						Desc("Machine-readable TPC-C report (tpm_c, per-transaction mix and response times, verdicts) when the workload emits one."),
					schemapb.Str("error").Title("Error").
						Desc("Failure text; set when the status is failed.").MaxLen(4096),
				).Strict().Rule(schemapb.Rule(
					`!("status" in this) || this.status != "failed" || ("error" in this)`,
					"a failed segment must carry an error",
				).ID("failed-segment-has-error")),
			).Title("Segments").Group("Result").
				Desc("One entry per workload segment, in execution order.").
				MaxItems(64),

			schemapb.List("artifacts", schemapb.Str("").MinLen(1).MaxLen(256)).
				Title("Artifacts").Group("Result").
				Desc("Graphene artifact references (artifact/<id>): raw stroppy output, the report.").
				MaxItems(2*64+1).Unique(),

			// doc: `stroppy baseline --json` — schema 1 report.
			schemapb.Object("baseline",
				schemapb.Bool("ok").Title("OK").
					Desc("No verdict failed; warnings do not clear it.").Required(),
				schemapb.List("verdicts",
					schemapb.Object("",
						schemapb.Str("check").Title("Check").MaxLen(128).Required(),
						schemapb.Choice("status").Title("Status").
							Opt(schemapb.StrV("ok"), "OK").
							Opt(schemapb.StrV("warn"), "Warning").
							Opt(schemapb.StrV("fail"), "Failed").
							Required(),
						schemapb.Str("detail").Title("Detail").MaxLen(1024),
					).Strict(),
				).Title("Verdicts").Desc("Hardware-independent invariants stroppy checked.").MaxItems(64),
				schemapb.JSON("report").Title("Report").
					Desc("The full baseline JSON report (host, tiers, verdicts)."),
				schemapb.Str("error").Title("Error").
					Desc("Why the baseline did not run to completion.").MaxLen(4096),
			).Title("Baseline").Group("Result").
				Desc("Runner machine self-check from `stroppy baseline`, when the workload asked for one.").Strict(),

			schemapb.Object("summary",
				schemapb.Double("tps").Title("Throughput").Unit("tps").
					Desc("Transactions per second over the measured segments.").Gte(0),
				schemapb.Double("latency_p50_ms").Title("Latency p50").Unit("ms").Gte(0),
				schemapb.Double("latency_p95_ms").Title("Latency p95").Unit("ms").Gte(0),
				schemapb.Double("latency_p99_ms").Title("Latency p99").Unit("ms").Gte(0),
				schemapb.Int64("errors").Title("Errors").
					Desc("Failed requests across the run.").Gte(0),
				schemapb.Duration("duration").Title("Duration").
					Desc("Wall-clock length of the measured part of the run.").Gte(0),
			).Title("Summary").Group("Summary").
				Desc("The headline numbers the run list, rating and compare sort on.").Strict(),
		).
		MustBuild()
}
