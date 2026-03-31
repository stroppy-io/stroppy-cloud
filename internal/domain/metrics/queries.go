package metrics

import (
	"fmt"
	"strings"
)

// RunLabel is the label injected by monitoring to identify a run.
const RunLabel = "stroppy_run_id"

// MetricDef defines a named PromQL query template.
// All templates accept a run_id filter.
type MetricDef struct {
	Name  string // human-readable name
	Key   string // stable key for comparison
	Query string // PromQL template with %s for run_id filter
	Unit  string // e.g. "ops/s", "ms", "bytes", "%"
}

func runFilter(runID string) string {
	return fmt.Sprintf(`%s="%s"`, RunLabel, runID)
}

// DefaultMetrics returns the standard set of metrics collected per run.
func DefaultMetrics() []MetricDef {
	return []MetricDef{
		// --- Database (standard postgres_exporter metrics) ---
		{
			Name:  "DB Transactions Per Second",
			Key:   "db_tps",
			Query: `sum(rate(pg_stat_database_xact_commit{%s}[1m]) + rate(pg_stat_database_xact_rollback{%s}[1m]))`,
			Unit:  "txn/s",
		},
		{
			Name:  "DB Rows Fetched Per Second",
			Key:   "db_qps",
			Query: `sum(rate(pg_stat_database_tup_fetched{%s}[1m]))`,
			Unit:  "rows/s",
		},
		{
			Name:  "DB Active Connections",
			Key:   "db_connections",
			Query: `sum(pg_stat_activity_count{%s})`,
			Unit:  "",
		},
		{
			Name:  "DB Replication Lag",
			Key:   "db_repl_lag",
			Query: `pg_stat_replication_pg_wal_lsn_diff{%s}`,
			Unit:  "bytes",
		},

		// --- System ---
		{
			Name:  "CPU Usage",
			Key:   "cpu_usage",
			Query: `100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle",%s}[1m])) * 100)`,
			Unit:  "%",
		},
		{
			Name:  "Memory Usage",
			Key:   "memory_usage",
			Query: `(1 - node_memory_MemAvailable_bytes{%s} / node_memory_MemTotal_bytes) * 100`,
			Unit:  "%",
		},
		{
			Name:  "Disk IO Read",
			Key:   "disk_read",
			Query: `rate(node_disk_read_bytes_total{%s}[1m])`,
			Unit:  "bytes/s",
		},
		{
			Name:  "Disk IO Write",
			Key:   "disk_write",
			Query: `rate(node_disk_written_bytes_total{%s}[1m])`,
			Unit:  "bytes/s",
		},
		{
			Name:  "Network Received",
			Key:   "net_rx",
			Query: `rate(node_network_receive_bytes_total{%s}[1m])`,
			Unit:  "bytes/s",
		},
		{
			Name:  "Network Transmitted",
			Key:   "net_tx",
			Query: `rate(node_network_transmit_bytes_total{%s}[1m])`,
			Unit:  "bytes/s",
		},

		// --- Stroppy (K6 OTEL metrics, matching stroppy-otel.json dashboard) ---
		// Note: stroppy K6 OTEL metrics use service.name label, not stroppy_run_id.
		{
			Name:  "Stroppy Active VUs",
			Key:   "stroppy_vus",
			Query: `sum(stroppy_vus{service_name="stroppy"})`,
			Unit:  "",
		},
		{
			Name:  "Stroppy Iterations/s",
			Key:   "stroppy_ops",
			Query: `sum(rate(stroppy_iterations_total{service_name="stroppy"}[30s]))`,
			Unit:  "iter/s",
		},
		{
			Name:  "Stroppy Iteration Duration p99",
			Key:   "stroppy_iter_p99",
			Query: `histogram_quantile(0.99, sum by (le) (rate(stroppy_iteration_duration_milliseconds_bucket{service_name="stroppy"}[30s])))`,
			Unit:  "ms",
		},
		{
			Name:  "Stroppy Query Rate",
			Key:   "stroppy_query_rate",
			Query: `sum(rate(stroppy_run_query_count_total{service_name="stroppy"}[30s]))`,
			Unit:  "q/s",
		},
		{
			Name:  "Stroppy Query Duration p99",
			Key:   "stroppy_latency_p99",
			Query: `histogram_quantile(0.99, sum by (le) (rate(stroppy_run_query_duration_milliseconds_bucket{service_name="stroppy"}[30s])))`,
			Unit:  "ms",
		},
		{
			Name:  "Stroppy Error Rate",
			Key:   "stroppy_errors",
			Query: `sum(rate(stroppy_run_query_error_rate_total{service_name="stroppy"}[30s]))`,
			Unit:  "errors/s",
		},
	}
}

// RenderQuery fills the run_id filter into all %s placeholders in a MetricDef query.
func RenderQuery(def MetricDef, runID string) string {
	f := runFilter(runID)
	return strings.ReplaceAll(def.Query, "%s", f)
}
