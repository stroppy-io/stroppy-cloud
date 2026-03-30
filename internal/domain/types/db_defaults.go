package types

// PostgresDefaults returns default postgresql.conf options by version.
// Based on CI perf testing best practices (metrics_runner.sh uses
// shared_buffers=25% RAM, max_wal_size=4GB, max_connections=200).
func PostgresDefaults(version string) map[string]string {
	base := map[string]string{
		"shared_buffers":               "25%", // 25% of total RAM, resolved by agent
		"max_connections":              "200",
		"max_wal_size":                 "4GB",
		"effective_cache_size":         "75%", // 75% of total RAM
		"work_mem":                     "64MB",
		"maintenance_work_mem":         "512MB",
		"wal_buffers":                  "64MB",
		"checkpoint_completion_target": "0.9",
		"random_page_cost":             "1.1", // SSD-optimized
		"effective_io_concurrency":     "200",
		"listen_addresses":             "'*'",
	}

	// Version-specific overrides can be added here.
	switch version {
	case "17":
		// PG 17 supports incremental backup natively; enable summarizer.
		base["summarize_wal"] = "on"
	}

	return base
}

// MySQLDefaults returns default my.cnf options by version.
func MySQLDefaults(version string) map[string]string {
	base := map[string]string{
		"innodb_buffer_pool_size":        "25%", // 25% of total RAM, resolved by agent
		"max_connections":                "200",
		"innodb_log_file_size":           "1G",
		"innodb_flush_method":            "O_DIRECT",
		"innodb_flush_log_at_trx_commit": "1",
		"innodb_io_capacity":             "2000", // SSD-optimized
		"innodb_io_capacity_max":         "4000",
		"innodb_read_io_threads":         "8",
		"innodb_write_io_threads":        "8",
		"table_open_cache":               "4000",
		"thread_cache_size":              "64",
		"bind_address":                   "0.0.0.0",
	}

	switch version {
	case "8.4":
		// MySQL 8.4 deprecates mysql_native_password; ensure caching_sha2 is default.
		base["default_authentication_plugin"] = "caching_sha2_password"
	}

	return base
}

// PicodataDefaults returns default picodata configuration options by version.
func PicodataDefaults(version string) map[string]string {
	return map[string]string{
		"replication_factor": "2",
		"shards":             "3",
		"memtx_memory":       "25%", // 25% of total RAM, resolved by agent
		"vinyl_memory":       "25%",
		"net_msg_max":        "1024",
		"readahead":          "16384",
		"listen":             "0.0.0.0:3301",
		"log_level":          "info",
	}
}
