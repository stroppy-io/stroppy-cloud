package metrics

import (
	"fmt"
	"strings"
)

// RunLabel is the label injected by monitoring (vmagent external_labels) to identify a run.
const RunLabel = "stroppy_run_id"

// MetricDef defines a named PromQL query template.
// Templates use %s for run_id label filter and %p for metric prefix (runID with dashes→underscores).
type MetricDef struct {
	Name  string // human-readable name
	Key   string // stable key for comparison
	Query string // PromQL template
	Unit  string // e.g. "ops/s", "ms", "bytes", "%"
}

func runFilter(runID string) string {
	return fmt.Sprintf(`%s="%s"`, RunLabel, runID)
}

// metricPrefix converts a run ID to a PromQL-safe metric prefix (dashes→underscores).
func metricPrefix(runID string) string {
	return strings.ReplaceAll(runID, "-", "_")
}

// DefaultMetrics returns the standard set of metrics collected per run.
func DefaultMetrics() []MetricDef {
	return []MetricDef{
		// --- Database (standard postgres_exporter metrics) ---
		// Filtered by stroppy_run_id label (set by vmagent external_labels).
		{
			Name:  "DB Transactions Per Second",
			Key:   "db_tps",
			Query: `sum(rate(pg_stat_database_xact_commit{%s}[5m]) + rate(pg_stat_database_xact_rollback{%s}[5m]))`,
			Unit:  "txn/s",
		},
		{
			Name:  "DB Rows Fetched Per Second",
			Key:   "db_qps",
			Query: `sum(rate(pg_stat_database_tup_fetched{%s}[5m]))`,
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
			Query: `max(pg_replication_lag_seconds{%s})`,
			Unit:  "s",
		},

		// --- System ---
		// Filtered by stroppy_run_id label (set by vmagent external_labels).
		{
			Name:  "CPU Usage",
			Key:   "cpu_usage",
			Query: `clamp_min(100 - (avg(rate(node_cpu_seconds_total{mode="idle",%s}[5m])) * 100), 0)`,
			Unit:  "%",
		},
		{
			Name:  "Memory Usage",
			Key:   "memory_usage",
			Query: `avg((1 - node_memory_MemAvailable_bytes{%s} / node_memory_MemTotal_bytes) * 100)`,
			Unit:  "%",
		},
		{
			Name:  "Disk IO Read",
			Key:   "disk_read",
			Query: `sum(rate(node_disk_read_bytes_total{%s}[5m]))`,
			Unit:  "bytes/s",
		},
		{
			Name:  "Disk IO Write",
			Key:   "disk_write",
			Query: `sum(rate(node_disk_written_bytes_total{%s}[5m]))`,
			Unit:  "bytes/s",
		},
		{
			Name:  "Network Received",
			Key:   "net_rx",
			Query: `sum(rate(node_network_receive_bytes_total{%s}[5m]))`,
			Unit:  "bytes/s",
		},
		{
			Name:  "Network Transmitted",
			Key:   "net_tx",
			Query: `sum(rate(node_network_transmit_bytes_total{%s}[5m]))`,
			Unit:  "bytes/s",
		},

		// --- Stroppy (K6 OTEL metrics, prefixed with runID_) ---
		// Metric names use %p prefix (runID with underscores).
		{
			Name:  "Stroppy Active VUs",
			Key:   "stroppy_vus",
			Query: `sum(%p_vus)`,
			Unit:  "",
		},
		{
			Name:  "Stroppy Iterations/s",
			Key:   "stroppy_ops",
			Query: `sum(rate(%p_iterations[30s]))`,
			Unit:  "iter/s",
		},
		{
			Name:  "Stroppy Iteration Duration p99",
			Key:   "stroppy_iter_p99",
			Query: `histogram_quantile(0.99, sum by (le) (rate(%p_iteration_duration_bucket[30s])))`,
			Unit:  "ms",
		},
		{
			Name:  "Stroppy Query Rate",
			Key:   "stroppy_query_rate",
			Query: `sum(rate(%p_run_query_count[30s]))`,
			Unit:  "q/s",
		},
		{
			Name:  "Stroppy Query Duration p99",
			Key:   "stroppy_latency_p99",
			Query: `histogram_quantile(0.99, sum by (le) (rate(%p_run_query_duration_bucket[30s])))`,
			Unit:  "ms",
		},
		{
			Name:  "Stroppy Error Count",
			Key:   "stroppy_errors",
			Query: `sum(%p_run_query_error_rate)`,
			Unit:  "",
		},
	}
}

// RenderQuery fills the run_id filter (%s) and metric prefix (%p) into a MetricDef query.
func RenderQuery(def MetricDef, runID string) string {
	q := def.Query
	q = strings.ReplaceAll(q, "%s", runFilter(runID))
	q = strings.ReplaceAll(q, "%p", metricPrefix(runID))
	return q
}
