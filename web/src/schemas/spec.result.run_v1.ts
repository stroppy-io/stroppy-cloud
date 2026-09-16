// GENERATED from schemapb schema spec.result.run@1 — do not edit.
// RunResult: headline metrics, per-segment outcome, artifacts and summary of one run.

/** map value metrics */
export interface SpecResultRun1MetricsValue {
  /** Value. Headline value of the metric over the window. */
  value: number;
  /** Unit. Unit of the value: tps, ms, ratio… */
  unit?: string;
  /** Minimum. Lowest sample in the window. */
  min?: number;
  /** Maximum. Highest sample in the window. */
  max?: number;
  /** Average. Mean over the window. */
  avg?: number;
}

/** map value metrics */
export interface SpecResultRun1ItemMetricsValue {
  /** Value. Headline value of the metric over the window. */
  value: number;
  /** Unit. Unit of the value: tps, ms, ratio… */
  unit?: string;
  /** Minimum. Lowest sample in the window. */
  min?: number;
  /** Maximum. Highest sample in the window. */
  max?: number;
  /** Average. Mean over the window. */
  avg?: number;
}

/** object errors */
export interface SpecResultRun1ItemErrors {
  /** Terminal errors. */
  terminal_errors?: number | string;
  /** Failed iterations. */
  failed_iterations?: number | string;
  /** Failed queries. */
  failed_queries?: number | string;
  /** Retry attempts. */
  retry_attempts?: number | string;
}

/** object  */
export interface SpecResultRun1Item {
  /** Segment. Name of the workload segment. */
  name: string;
  /** Status. How the segment ended. */
  status: "completed" | "failed" | "skipped" | "canceled";
  /** Started. */
  started_at?: string;
  /** Finished. */
  finished_at?: string;
  /** Metrics. Metrics measured over this segment's window only (bench summary counters and histogram statistics). */
  metrics?: Record<string, SpecResultRun1ItemMetricsValue>;
  /** Nonfatal errors. Stroppy's own error accounting; nonfatal errors keep the exit status 0. */
  errors?: SpecResultRun1ItemErrors;
  /** Exit code. Exit status of the stroppy process (0 ok, 130/143 canceled, 1 error). */
  exit_code?: number | string;
  /** TPC-C compliance report. Machine-readable TPC-C report (tpm_c, per-transaction mix and response times, verdicts) when the workload emits one. */
  compliance?: unknown;
  /** Error. Failure text; set when the status is failed. */
  error?: string;
}

/** object  */
export interface SpecResultRun1BaselineItem {
  /** Check. */
  check: string;
  /** Status. */
  status: "ok" | "warn" | "fail";
  /** Detail. */
  detail?: string;
}

/** object baseline */
export interface SpecResultRun1Baseline {
  /** OK. No verdict failed; warnings do not clear it. */
  ok: boolean;
  /** Verdicts. Hardware-independent invariants stroppy checked. */
  verdicts?: Array<SpecResultRun1BaselineItem>;
  /** Report. The full baseline JSON report (host, tiers, verdicts). */
  report?: unknown;
  /** Error. Why the baseline did not run to completion. */
  error?: string;
}

/** object summary */
export interface SpecResultRun1Summary {
  /** Throughput. Transactions per second over the measured segments. [tps] */
  tps?: number;
  /** Latency p50. [ms] */
  latency_p50_ms?: number;
  /** Latency p95. [ms] */
  latency_p95_ms?: number;
  /** Latency p99. [ms] */
  latency_p99_ms?: number;
  /** Errors. Failed requests across the run. */
  errors?: number | string;
  /** Duration. Wall-clock length of the measured part of the run. */
  duration?: string;
}

/** root */
export interface SpecResultRun1 {
  /** Metrics. Metrics keyed by segment and metric name; up to 256 metrics for each of 64 segments. */
  metrics?: Record<string, SpecResultRun1MetricsValue>;
  /** Segments. One entry per workload segment, in execution order. */
  segments?: Array<SpecResultRun1Item>;
  /** Artifacts. Graphene artifact references (artifact/<id>): raw stroppy output, the report. */
  artifacts?: Array<string>;
  /** Baseline. Runner machine self-check from `stroppy baseline`, when the workload asked for one. */
  baseline?: SpecResultRun1Baseline;
  /** Summary. The headline numbers the run list, rating and compare sort on. */
  summary?: SpecResultRun1Summary;
}
