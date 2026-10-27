package compile_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const mysqlConf = "/etc/mysql/conf.d/zz-stroppy.cnf"

// innodbSchema is the my.cnf schema of a MySQL/MariaDB version.
func innodbSchema(kind catalog.DatabaseKind, version string) string {
	if kind == catalog.MariaDB {
		return "cfg.mariadb.cnf@" + version
	}
	return "cfg.my.cnf@" + strings.TrimSuffix(version, ".0")
}

// The auto profile on the S/M/L machines (4/16/100 network-ssd, 8/32/200
// network-ssd, 16/64/400 io-m3) of every MySQL and MariaDB version: the
// rendered my.cnf and the run's effective configs carry the same numbers.
func TestInnoDBAutoBySize(t *testing.T) {
	type want map[string]any
	mysql := map[string]want{
		// pool 70% rounded down to 128M x instances; redo pool/4; io from
		// the disk (S: 4000 IOPS / 60 MB/s, M: 7000 / 105, L: 28000 / 410).
		"S": {"innodb_buffer_pool_size": 11264, "innodb_buffer_pool_instances": 2, "innodb_redo_log_capacity": 2816,
			"innodb_io_capacity": 1500, "innodb_io_capacity_max": 2600, "innodb_page_cleaners": 2, "innodb_purge_threads": 2},
		"M": {"innodb_buffer_pool_size": 22528, "innodb_buffer_pool_instances": 4, "innodb_redo_log_capacity": 5632,
			"innodb_io_capacity": 2600, "innodb_io_capacity_max": 4700, "innodb_page_cleaners": 4, "innodb_purge_threads": 4},
		"L": {"innodb_buffer_pool_size": 45056, "innodb_buffer_pool_instances": 8, "innodb_redo_log_capacity": 11264,
			"innodb_io_capacity": 10400, "innodb_io_capacity_max": 18300, "innodb_page_cleaners": 8, "innodb_purge_threads": 4},
	}
	maria := map[string]want{
		"S": {"innodb_buffer_pool_size": 11468, "innodb_log_file_size": 2867, "innodb_io_capacity": 1500, "innodb_io_capacity_max": 2600},
		"M": {"innodb_buffer_pool_size": 22937, "innodb_log_file_size": 5734, "innodb_io_capacity": 2600, "innodb_io_capacity_max": 4700},
		"L": {"innodb_buffer_pool_size": 45875, "innodb_log_file_size": 11468, "innodb_io_capacity": 10400, "innodb_io_capacity_max": 18300},
	}
	common := want{"innodb_log_buffer_size": 64, "skip_name_resolve": "ON", "binlog_expire_logs_seconds": 86400}
	mysqlCommon := want{"max_connections": 500, "open_files_limit": 65535, "innodb_adaptive_hash_index": "OFF",
		"innodb_change_buffering": "none", "innodb_use_fdatasync": "ON", "innodb_flush_neighbors": 0}
	mariaCommon := want{"max_connections": 300, "sync_binlog": 1}
	for _, kind := range []catalog.DatabaseKind{catalog.MySQL, catalog.MariaDB} {
		versions := []string{"8.0", "8.4"}
		table, extra := mysql, mysqlCommon
		if kind == catalog.MariaDB {
			versions, table, extra = []string{"10.11", "11.4", "11.8"}, maria, mariaCommon
		}
		for _, version := range versions {
			for _, size := range []string{"S", "M", "L"} {
				t.Run(fmt.Sprintf("%s/%s/%s", kind, version, size), func(t *testing.T) {
					out, eff := compileTuned(t, tuneCase{kind: kind, version: version, protocol: catalog.ProtoMySQL, size: size})
					schemaID := innodbSchema(kind, version)
					conf := fileOf(out, "-"+string(kind), mysqlConf)
					values := effectiveOf(t, eff, "db", schemaID)
					for _, w := range []want{table[size], common, extra} {
						for key, v := range w {
							if fmt.Sprint(values[key]) != fmt.Sprint(v) {
								t.Errorf("effective %s = %v, want %v", key, values[key], v)
							}
							unit := ""
							if strings.HasSuffix(key, "_size") || key == "innodb_redo_log_capacity" {
								unit = "M"
							}
							if line := fmt.Sprintf("\n%s = %v%s\n", key, v, unit); !strings.Contains(conf, line) {
								t.Errorf("my.cnf lacks %q", strings.TrimSpace(line))
							}
						}
					}
					if version == "11.8" && !strings.Contains(conf, "\ninnodb_snapshot_isolation = ON\n") {
						t.Error("11.8 keeps its snapshot isolation default")
					}
				})
			}
		}
	}
}

// A key set in the role's config wins over auto, and values derived
// from it follow the explicit value.
func TestInnoDBAutoExplicitKeyWins(t *testing.T) {
	out, eff := compileTuned(t, tuneCase{
		kind: catalog.MySQL, version: "8.4", protocol: catalog.ProtoMySQL,
		configs: map[string]map[string]json.RawMessage{"db": {"cfg.my.cnf@8.4": json.RawMessage(`{"innodb_buffer_pool_size":8192,"innodb_io_capacity":20000,"max_connections":222}`)}},
	})
	conf := fileOf(out, "-mysql", mysqlConf)
	for _, line := range []string{
		"innodb_buffer_pool_size = 8192M", "innodb_io_capacity = 20000", "max_connections = 222",
		// Derived from the explicit pool: 8 GB / 4 = 2 GB redo.
		"innodb_redo_log_capacity = 2048M",
		// Kept above the explicit io_capacity.
		"innodb_io_capacity_max = 20000",
	} {
		if !strings.Contains(conf, "\n"+line+"\n") {
			t.Errorf("my.cnf lacks %q:\n%s", line, conf)
		}
	}
	tuned := out.Tuned["db"]["cfg.my.cnf@8.4"]
	for _, key := range []string{"innodb_buffer_pool_size", "innodb_io_capacity", "max_connections"} {
		if _, ok := tuned[key]; ok {
			t.Errorf("auto recorded the explicit key %s", key)
		}
	}
	if v := effectiveOf(t, eff, "db", "cfg.my.cnf@8.4")["max_connections"]; v != float64(222) {
		t.Errorf("effective max_connections = %v", v)
	}
}

// stock (and the legacy buffer_pool: stock) renders the schema defaults.
func TestInnoDBStockUnchanged(t *testing.T) {
	for _, params := range []map[string]any{{"tuning": "stock"}, {"buffer_pool": "stock"}} {
		for _, kind := range []catalog.DatabaseKind{catalog.MySQL, catalog.MariaDB} {
			version := "8.4"
			lines := []string{"innodb_buffer_pool_size = 128M", "innodb_redo_log_capacity = 100M", "innodb_io_capacity = 10000", "max_connections = 151", "skip_name_resolve = OFF"}
			if kind == catalog.MariaDB {
				version = "11.4"
				lines = []string{"innodb_buffer_pool_size = 128M", "innodb_log_file_size = 96M", "innodb_io_capacity = 200", "sync_binlog = 0", "max_connections = 151"}
			}
			t.Run(fmt.Sprint(kind, params), func(t *testing.T) {
				out, _ := compileTuned(t, tuneCase{kind: kind, version: version, protocol: catalog.ProtoMySQL, params: params})
				conf := fileOf(out, "-"+string(kind), mysqlConf)
				for _, line := range lines {
					if !strings.Contains(conf, "\n"+line+"\n") {
						t.Errorf("stock my.cnf lacks %q:\n%s", line, conf)
					}
				}
				if strings.Contains(conf, "innodb_buffer_pool_instances") {
					t.Error("stock leaves the pool instances to the server")
				}
				if len(out.Tuned) != 0 {
					t.Errorf("stock recorded auto values: %v", out.Tuned)
				}
				for _, ct := range out.Spec.Containers {
					if ct.Kind == spec.ContainerKindDatabase && (len(ct.SecurityOpt) > 0 || ct.Ulimits["memlock"] != 0) {
						t.Errorf("%s: stock changed the container", ct.Name)
					}
				}
			})
		}
	}
}

// MariaDB semi_sync really is semi-synchronous: the built-in plugin is on
// on both sides and waits after sync.
func TestMariaDBSemiSyncEnabled(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.MariaDB, version: "11.4", protocol: catalog.ProtoMySQL,
		params: map[string]any{"replicas": 1, "replication": "semi_sync"}})
	primary, replica := "", ""
	for _, ct := range out.Spec.Containers {
		for _, f := range ct.Files {
			if f.Path == mysqlConf {
				if ct.Role == "db" {
					primary = f.Content
				} else {
					replica = f.Content
				}
			}
		}
	}
	for _, line := range []string{"rpl_semi_sync_master_enabled = ON", "rpl_semi_sync_master_wait_point = AFTER_SYNC", "rpl_semi_sync_slave_enabled = OFF"} {
		if !strings.Contains(primary, "\n"+line+"\n") {
			t.Errorf("primary lacks %q", line)
		}
	}
	for _, line := range []string{"rpl_semi_sync_slave_enabled = ON", "rpl_semi_sync_master_enabled = OFF", "read_only = ON"} {
		if !strings.Contains(replica, "\n"+line+"\n") {
			t.Errorf("replica lacks %q", line)
		}
	}
}

// MySQL semi-sync waits after sync; replicas are super_read_only from the
// real server's start (persisted, so the image's init server can write).
func TestMySQLSemiSyncReplica(t *testing.T) {
	out, eff := compileTuned(t, tuneCase{kind: catalog.MySQL, version: "8.0", protocol: catalog.ProtoMySQL,
		params: map[string]any{"replicas": 1, "replication": "semi_sync"}})
	if conf := fileOf(out, "db-1-mysql", mysqlConf); !strings.Contains(conf, "loose-rpl_semi_sync_source_wait_point = AFTER_SYNC\n") {
		t.Errorf("primary semi-sync wait point:\n%s", conf)
	}
	var replica spec.Container
	for _, ct := range out.Spec.Containers {
		if ct.Role == "db-replica" && ct.Kind == spec.ContainerKindDatabase {
			replica = ct
		}
	}
	conf, init := "", ""
	for _, f := range replica.Files {
		switch f.Path {
		case mysqlConf:
			conf = f.Content
		case "/docker-entrypoint-initdb.d/10-stroppy.sql":
			init = f.Content
		}
	}
	if !strings.Contains(conf, "\nsuper_read_only = OFF\n") || !strings.Contains(init, "SET PERSIST super_read_only = ON;") {
		t.Errorf("super_read_only must be persisted, not in my.cnf:\n%s\n%s", conf, init)
	}
	if !strings.Contains(conf, "\nbinlog_transaction_dependency_tracking = WRITESET\n") {
		t.Error("8.0 replication without the group block needs WRITESET")
	}
	if v := effectiveOf(t, eff, "db-replica", "cfg.my.cnf@8")["super_read_only"]; v != "ON" {
		t.Errorf("effective super_read_only = %v", v)
	}
}

// Group Replication renders one consistency and exit action on both
// versions (the preset's BEFORE is kept).
func TestMySQLGroupReplicationExplicit(t *testing.T) {
	for _, version := range []string{"8.0", "8.4"} {
		out, _ := compileTuned(t, tuneCase{kind: catalog.MySQL, version: version, protocol: catalog.ProtoMySQL,
			params: map[string]any{"replicas": 2, "replication": "group", "proxysql": 1}})
		conf := fileOf(out, "db-1-mysql", mysqlConf)
		for _, line := range []string{"loose-group_replication_consistency = BEFORE", "loose-group_replication_exit_state_action = READ_ONLY"} {
			if !strings.Contains(conf, line+"\n") {
				t.Errorf("%s lacks %q", version, line)
			}
		}
	}
}

// Galera: no global sync wait, the provider options and one applier per
// vCPU; ProxySQL sized to its machine and the workload, everything on the
// writer; the MariaDB containers may use io_uring.
func TestGaleraAuto(t *testing.T) {
	out, eff := compileTuned(t, tuneCase{kind: catalog.MariaDB, version: "11.4", protocol: catalog.ProtoMySQL,
		params: map[string]any{"replication": "galera", "galera_nodes": 3, "proxysql": 1}})
	conf := fileOf(out, "db-1-mariadb", mysqlConf)
	for _, line := range []string{
		"wsrep_sync_wait = 0", "wsrep_slave_threads = 8",
		`wsrep_provider_options = "gcache.size=2G; gcs.fc_limit=128; gcs.fc_factor=0.8; gcs.fc_single_primary=yes; evs.suspect_timeout=PT10S; evs.inactive_timeout=PT30S; evs.install_timeout=PT15S"`,
	} {
		if !strings.Contains(conf, "\n"+line+"\n") {
			t.Errorf("galera node lacks %q:\n%s", line, conf)
		}
	}
	if v := effectiveOf(t, eff, "db", "cfg.mariadb.cnf@11.4")["wsrep_sync_wait"]; v != float64(0) {
		t.Errorf("effective wsrep_sync_wait = %v", v)
	}
	proxy := fileOf(out, "-proxysql", "/etc/proxysql.cnf")
	// M proxy = 4 vCPU; backend max_connections 300 less headroom.
	for _, want := range []string{"threads=4\n", "max_connections=2048\n", "max_connections=250,", "writer_is_also_reader=1", `server_version="11.4.0-MariaDB"`} {
		if !strings.Contains(proxy, want) {
			t.Errorf("proxysql.cnf lacks %q:\n%s", want, proxy)
		}
	}
	if strings.Contains(proxy, "mysql_query_rules") {
		t.Error("Galera reads must not be split off the writer")
	}
	nodes := 0
	for _, ct := range out.Spec.Containers {
		if ct.Kind != spec.ContainerKindDatabase {
			continue
		}
		nodes++
		if ct.Ulimits["memlock"] != -1 || strings.Join(ct.SecurityOpt, ",") != "seccomp=unconfined" || ct.Ulimits["nofile"] != 1048576 {
			t.Errorf("%s: ulimits %v, security %v", ct.Name, ct.Ulimits, ct.SecurityOpt)
		}
	}
	if nodes != 3 {
		t.Fatalf("%d galera nodes", nodes)
	}
}

// The server and the proxy take the client's pool: 1000 VUs on 32-vCPU
// members share 8 × 32 = 256 connections, under MySQL's floor of 500.
func TestInnoDBConnectionsFollowThePool(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.MySQL, version: "8.4", protocol: catalog.ProtoMySQL, vus: 1000, size: "XL",
		params: map[string]any{"replicas": 2, "replication": "group", "proxysql": 1}})
	if conf := fileOf(out, "db-1-mysql", mysqlConf); !strings.Contains(conf, "\nmax_connections = 500\n") {
		t.Errorf("1000 VUs: %s", conf)
	}
	proxy := fileOf(out, "-proxysql", "/etc/proxysql.cnf")
	for _, want := range []string{"max_connections=2048\n", "max_connections=450,"} {
		if !strings.Contains(proxy, want) {
			t.Errorf("proxysql.cnf lacks %q:\n%s", want, proxy)
		}
	}
}
