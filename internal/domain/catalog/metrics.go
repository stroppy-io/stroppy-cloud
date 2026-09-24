package catalog

// agent groups a component series by the machine it was scraped on.
const agent = `"graphene.agent"`

// metrics is the metric catalog: the numbers of a run's result (keys of
// spec.result.run@1 as the pipeline's summary parser names them), and the
// time series the run's telemetry carries — Stroppy's native workload
// series, node_exporter on every machine and each database's exporter —
// as MetricsQL over the real series names.
func metrics() []Metric {
	pg := []DatabaseKind{Postgres, OrioleDB, PgNoop}
	mysql := []DatabaseKind{MySQL, MariaDB}
	quantile := func(q string) string {
		return `histogram_quantile(` + q + `, sum by (le) (rate(stroppy_iteration_duration_milliseconds_bucket{$native}[1m])))`
	}
	return []Metric{
		{
			// Stroppy counts a transaction only where the workload has
			// them: an iteration is not one (a TPC-C iteration is one
			// business transaction, a query-set iteration is none), so
			// the series is the transaction counter, not the iteration
			// counter, and it is empty for workloads that commit nothing.
			Key: "tps", Title: "Throughput", Description: "Committed transactions per second; workloads that run no transactions (query sets) report none.", Unit: "tps", HigherIsBetter: true, Group: "Headline", Scope: "result", RatingEligible: true,
			Expr: `sum(rate(stroppy_successful_transactions_total{$native}[1m]))`,
		},
		{Key: "latency_p50_ms", Title: "Latency p50", Unit: "ms", Group: "Headline", Scope: "result", RatingEligible: true, Expr: quantile("0.5")},
		{Key: "latency_p95_ms", Title: "Latency p95", Unit: "ms", Group: "Headline", Scope: "result", RatingEligible: true, Expr: quantile("0.95")},
		{Key: "latency_p99_ms", Title: "Latency p99", Unit: "ms", Group: "Headline", Scope: "result", RatingEligible: true, Expr: quantile("0.99")},
		{
			Key: "errors", Title: "Errors", Description: "Failed iterations and queries.", Unit: "count", Group: "Headline", Scope: "result",
			Expr: `sum(stroppy_failed_iterations_total{$native}) + sum(stroppy_failed_queries_total{$native})`,
		},
		{Key: "iterations_total", Title: "Iterations", Unit: "count", HigherIsBetter: true, Group: "Workload", Scope: "result", Expr: `sum(stroppy_iterations_total{$native})`},
		{
			Key: "iterations_per_second", Title: "Iterations per second", Description: "Workload iterations completed per second, whatever an iteration does.", Unit: "1/s", HigherIsBetter: true, Group: "Workload", Scope: "result",
			Expr: `sum(rate(stroppy_iterations_total{$native}[1m]))`,
		},
		{
			Key: "queries_per_second", Title: "Queries per second", Description: "Statements the workload sent to the database per second.", Unit: "1/s", HigherIsBetter: true, Group: "Workload", Scope: "result",
			Expr: `sum(rate(stroppy_run_query_operations_total{$native}[1m]))`,
		},
		{Key: "failed_iterations_total", Title: "Failed iterations", Unit: "count", Group: "Workload", Scope: "result", Expr: `sum(stroppy_failed_iterations_total{$native})`},
		{Key: "failed_queries_total", Title: "Failed queries", Unit: "count", Group: "Workload", Scope: "result", Expr: `sum(stroppy_failed_queries_total{$native})`},
		{
			Key: "iteration_duration_avg", Title: "Iteration duration, avg", Unit: "ms", Group: "Workload", Scope: "result",
			Expr: `sum(rate(stroppy_iteration_duration_milliseconds_sum{$native}[1m])) / sum(rate(stroppy_iteration_duration_milliseconds_count{$native}[1m]))`,
		},
		{Key: "iteration_duration_p90", Title: "Iteration duration, p90", Unit: "ms", Group: "Workload", Scope: "result", Expr: quantile("0.9")},
		{Key: "iteration_duration_p99", Title: "Iteration duration, p99", Unit: "ms", Group: "Workload", Scope: "result", Expr: quantile("0.99")},
		{Key: "load_duration_seconds", Title: "Data load duration", Unit: "s", Group: "Bootstrap", Scope: "result"},

		{
			Key: "node_cpu_usage", Title: "CPU usage", Unit: "%", Group: "Host", Scope: "host",
			Expr: `100 * (1 - avg by (` + agent + `) (rate(node_cpu_seconds_total{$run,mode="idle"}[1m])))`,
		},
		{
			Key: "node_memory_used_bytes", Title: "Memory used", Unit: "bytes", Group: "Host", Scope: "host",
			Expr: `sum by (` + agent + `) (node_memory_MemTotal_bytes{$run}) - sum by (` + agent + `) (node_memory_MemAvailable_bytes{$run})`,
		},
		{
			Key: "node_disk_io_bytes", Title: "Disk I/O", Unit: "bytes/s", Group: "Host", Scope: "host",
			Expr: `sum by (` + agent + `) (rate(node_disk_read_bytes_total{$run}[1m])) + sum by (` + agent + `) (rate(node_disk_written_bytes_total{$run}[1m]))`,
		},
		{
			Key: "node_network_bytes", Title: "Network", Unit: "bytes/s", Group: "Host", Scope: "host",
			Expr: `sum by (` + agent + `) (rate(node_network_receive_bytes_total{$run,device!="lo"}[1m])) + sum by (` + agent + `) (rate(node_network_transmit_bytes_total{$run,device!="lo"}[1m]))`,
		},

		{
			Key: "pg_stat_database_xact_commit", Title: "Commits", Unit: "1/s", HigherIsBetter: true, Group: "PostgreSQL", Scope: "db", DBKinds: pg,
			Expr: `sum by (` + agent + `) (rate(pg_stat_database_xact_commit{$run}[1m]))`,
		},
		{
			Key: "pg_stat_database_xact_rollback", Title: "Rollbacks", Unit: "1/s", Group: "PostgreSQL", Scope: "db", DBKinds: pg,
			Expr: `sum by (` + agent + `) (rate(pg_stat_database_xact_rollback{$run}[1m]))`,
		},
		{
			Key: "pg_stat_database_blks_hit_ratio", Title: "Buffer cache hit ratio", Unit: "%", HigherIsBetter: true, Group: "PostgreSQL", Scope: "db", DBKinds: pg,
			Expr: `100 * sum by (` + agent + `) (rate(pg_stat_database_blks_hit{$run}[1m])) / (sum by (` + agent + `) (rate(pg_stat_database_blks_hit{$run}[1m])) + sum by (` + agent + `) (rate(pg_stat_database_blks_read{$run}[1m])))`,
		},
		{
			Key: "pg_stat_activity_count", Title: "Backends", Unit: "count", Group: "PostgreSQL", Scope: "db", DBKinds: pg,
			Expr: `sum by (` + agent + `) (pg_stat_activity_count{$run})`,
		},
		{
			Key: "pg_locks_count", Title: "Locks", Unit: "count", Group: "PostgreSQL", Scope: "db", DBKinds: pg,
			Expr: `sum by (` + agent + `) (pg_locks_count{$run})`,
		},
		{
			Key: "pg_replication_lag_seconds", Title: "Replication lag", Unit: "s", Group: "PostgreSQL", Scope: "db", DBKinds: pg,
			Expr: `max by (` + agent + `) (pg_replication_lag_seconds{$run})`,
		},

		{
			Key: "mysql_global_status_queries", Title: "Queries", Unit: "1/s", HigherIsBetter: true, Group: "MySQL", Scope: "db", DBKinds: mysql,
			Expr: `sum by (` + agent + `) (rate(mysql_global_status_queries{$run}[1m]))`,
		},
		{
			Key: "mysql_global_status_threads_connected", Title: "Threads connected", Unit: "count", Group: "MySQL", Scope: "db", DBKinds: mysql,
			Expr: `sum by (` + agent + `) (mysql_global_status_threads_connected{$run})`,
		},
		{
			Key: "mysql_global_status_innodb_buffer_pool_hit_ratio", Title: "InnoDB buffer pool hit ratio", Unit: "%", HigherIsBetter: true, Group: "MySQL", Scope: "db", DBKinds: mysql,
			Expr: `100 * (1 - sum by (` + agent + `) (rate(mysql_global_status_innodb_buffer_pool_reads{$run}[1m])) / sum by (` + agent + `) (rate(mysql_global_status_innodb_buffer_pool_read_requests{$run}[1m])))`,
		},
		{
			Key: "mysql_slave_status_seconds_behind_master", Title: "Replica lag", Unit: "s", Group: "MySQL", Scope: "db", DBKinds: mysql,
			Expr: `max by (` + agent + `) (mysql_slave_status_seconds_behind_master{$run})`,
		},

		{
			Key: "cockroach_sql_txn_commit_count", Title: "Transactions committed", Unit: "1/s", HigherIsBetter: true, Group: "CockroachDB", Scope: "db", DBKinds: []DatabaseKind{Cockroach},
			Expr: `sum by (` + agent + `) (rate(sql_txn_commit_count{$run}[1m]))`,
		},
		{
			Key: "cockroach_sql_service_latency_p99", Title: "SQL latency p99", Unit: "ms", Group: "CockroachDB", Scope: "db", DBKinds: []DatabaseKind{Cockroach},
			Expr: `histogram_quantile(0.99, sum by (le) (rate(sql_service_latency_bucket{$run}[1m]))) / 1e6`,
		},
		{
			Key: "cockroach_live_nodes", Title: "Live nodes", Unit: "count", HigherIsBetter: true, Group: "CockroachDB", Scope: "db", DBKinds: []DatabaseKind{Cockroach},
			Expr: `max(liveness_livenodes{$run})`,
		},
		{
			Key: "picodata_instances_online", Title: "Instances online", Unit: "count", HigherIsBetter: true, Group: "Picodata", Scope: "db", DBKinds: []DatabaseKind{Picodata},
			Expr: `count(pico_instance_state{$run,state="Online"} == 1)`,
		},
	}
}
