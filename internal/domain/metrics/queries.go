package metrics

import "fmt"

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
		// --- Database ---
		{
			Name:  "DB Queries Per Second",
			Key:   "db_qps",
			Query: `rate(pg_stat_statements_calls_total{%s}[1m])`,
			Unit:  "ops/s",
		},
		{
			Name:  "DB Query Latency p99",
			Key:   "db_latency_p99",
			Query: `histogram_quantile(0.99, rate(pg_stat_statements_seconds_bucket{%s}[5m]))`,
			Unit:  "s",
		},
		{
			Name:  "DB Active Connections",
			Key:   "db_connections",
			Query: `pg_stat_activity_count{%s}`,
			Unit:  "",
		},
		{
			Name:  "DB Replication Lag",
			Key:   "db_repl_lag",
			Query: `pg_replication_lag{%s}`,
			Unit:  "s",
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
			Query: `(1 - node_memory_MemAvailable_bytes{%s} / node_memory_MemTotal_bytes{%s}) * 100`,
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

		// --- Stroppy ---
		{
			Name:  "Stroppy Operations/s",
			Key:   "stroppy_ops",
			Query: `rate(stroppy_operations_total{%s}[1m])`,
			Unit:  "ops/s",
		},
		{
			Name:  "Stroppy Latency p99",
			Key:   "stroppy_latency_p99",
			Query: `histogram_quantile(0.99, rate(stroppy_operation_duration_seconds_bucket{%s}[5m]))`,
			Unit:  "s",
		},
		{
			Name:  "Stroppy Error Rate",
			Key:   "stroppy_errors",
			Query: `rate(stroppy_errors_total{%s}[1m])`,
			Unit:  "errors/s",
		},
	}
}

// RenderQuery fills the run_id filter into a MetricDef query.
func RenderQuery(def MetricDef, runID string) string {
	f := runFilter(runID)
	return fmt.Sprintf(def.Query, f)
}
