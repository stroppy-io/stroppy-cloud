package types

import (
	"testing"
)

func TestPostgresDefaults_BaseKeys(t *testing.T) {
	d := PostgresDefaults("16")

	expectedKeys := []string{
		"shared_buffers",
		"max_connections",
		"max_wal_size",
		"effective_cache_size",
		"work_mem",
		"maintenance_work_mem",
		"wal_buffers",
		"checkpoint_completion_target",
		"random_page_cost",
		"effective_io_concurrency",
		"listen_addresses",
	}

	for _, k := range expectedKeys {
		if _, ok := d[k]; !ok {
			t.Errorf("PostgresDefaults(16) missing key %q", k)
		}
	}
}

func TestPostgresDefaults_Version17HasSummarizeWal(t *testing.T) {
	d := PostgresDefaults("17")
	if d["summarize_wal"] != "on" {
		t.Errorf("PostgresDefaults(17) should have summarize_wal=on, got %q", d["summarize_wal"])
	}
}

func TestPostgresDefaults_Version16NoSummarizeWal(t *testing.T) {
	d := PostgresDefaults("16")
	if _, ok := d["summarize_wal"]; ok {
		t.Error("PostgresDefaults(16) should not have summarize_wal")
	}
}

func TestPostgresDefaults_PercentagePlaceholders(t *testing.T) {
	d := PostgresDefaults("16")
	if d["shared_buffers"] != "25%" {
		t.Errorf("expected shared_buffers=25%%, got %s", d["shared_buffers"])
	}
	if d["effective_cache_size"] != "75%" {
		t.Errorf("expected effective_cache_size=75%%, got %s", d["effective_cache_size"])
	}
}

func TestMySQLDefaults_BaseKeys(t *testing.T) {
	d := MySQLDefaults("8.0")

	expectedKeys := []string{
		"innodb_buffer_pool_size",
		"max_connections",
		"innodb_log_file_size",
		"innodb_flush_method",
		"innodb_flush_log_at_trx_commit",
		"innodb_io_capacity",
		"innodb_io_capacity_max",
		"innodb_read_io_threads",
		"innodb_write_io_threads",
		"table_open_cache",
		"thread_cache_size",
		"bind_address",
	}

	for _, k := range expectedKeys {
		if _, ok := d[k]; !ok {
			t.Errorf("MySQLDefaults(8.0) missing key %q", k)
		}
	}
}

func TestMySQLDefaults_Version84HasCachingSha2(t *testing.T) {
	d := MySQLDefaults("8.4")
	if d["default_authentication_plugin"] != "caching_sha2_password" {
		t.Errorf("MySQLDefaults(8.4) should have caching_sha2_password, got %q", d["default_authentication_plugin"])
	}
}

func TestMySQLDefaults_Version80NoCachingSha2(t *testing.T) {
	d := MySQLDefaults("8.0")
	if _, ok := d["default_authentication_plugin"]; ok {
		t.Error("MySQLDefaults(8.0) should not have default_authentication_plugin")
	}
}

func TestMySQLDefaults_PercentagePlaceholder(t *testing.T) {
	d := MySQLDefaults("8.0")
	if d["innodb_buffer_pool_size"] != "25%" {
		t.Errorf("expected innodb_buffer_pool_size=25%%, got %s", d["innodb_buffer_pool_size"])
	}
}

func TestPicodataDefaults_BaseKeys(t *testing.T) {
	d := PicodataDefaults("25.3")

	expectedKeys := []string{
		"replication_factor",
		"shards",
		"memtx_memory",
		"vinyl_memory",
		"net_msg_max",
		"readahead",
		"listen",
		"log_level",
	}

	for _, k := range expectedKeys {
		if _, ok := d[k]; !ok {
			t.Errorf("PicodataDefaults(25.3) missing key %q", k)
		}
	}
}

func TestPicodataDefaults_PercentagePlaceholders(t *testing.T) {
	d := PicodataDefaults("25.3")
	if d["memtx_memory"] != "25%" {
		t.Errorf("expected memtx_memory=25%%, got %s", d["memtx_memory"])
	}
	if d["vinyl_memory"] != "25%" {
		t.Errorf("expected vinyl_memory=25%%, got %s", d["vinyl_memory"])
	}
}

func TestPicodataDefaults_ListenAddress(t *testing.T) {
	d := PicodataDefaults("25.3")
	if d["listen"] != "0.0.0.0:3301" {
		t.Errorf("expected listen=0.0.0.0:3301, got %s", d["listen"])
	}
}

func TestPicodataDefaults_UnknownVersionStillReturnsDefaults(t *testing.T) {
	d := PicodataDefaults("99.0")
	// PicodataDefaults has no version-specific logic, so any version returns the base.
	if len(d) == 0 {
		t.Error("PicodataDefaults should return non-empty map for any version")
	}
}

func TestPostgresDefaults_UnknownVersionStillReturnsBase(t *testing.T) {
	d := PostgresDefaults("99")
	if len(d) == 0 {
		t.Error("PostgresDefaults should return non-empty map for unknown version")
	}
	// Should not have version-17-specific keys.
	if _, ok := d["summarize_wal"]; ok {
		t.Error("unknown version should not have summarize_wal")
	}
}

func TestMySQLDefaults_UnknownVersionStillReturnsBase(t *testing.T) {
	d := MySQLDefaults("99")
	if len(d) == 0 {
		t.Error("MySQLDefaults should return non-empty map for unknown version")
	}
	if _, ok := d["default_authentication_plugin"]; ok {
		t.Error("unknown version should not have default_authentication_plugin")
	}
}
