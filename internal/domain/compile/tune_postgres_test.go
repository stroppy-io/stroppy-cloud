package compile_test

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// compileOriole is compileTuned for OrioleDB, whose params name the image
// tag instead of a version.
func compileOriole(t *testing.T, tc tuneCase) (compile.Output, map[string]map[string]json.RawMessage) {
	t.Helper()
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	params := map[string]any{"image_tag": tc.version}
	for k, v := range tc.params {
		params[k] = v
	}
	raw, _ := json.Marshal(params)
	dspec, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: catalog.OrioleDB, Version: tc.version, Params: raw, Configs: tc.configs})
	if err != nil {
		t.Fatal(err)
	}
	segment := `{"name":"main","workload":{"script":"tpcc/tx","scale_factor":100},"run":{"vus":128,"duration":"10m"}}`
	wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: catalog.ProtoPg, Segments: []json.RawMessage{json.RawMessage(segment)}})
	if err != nil {
		t.Fatal(err)
	}
	size := tc.size
	if size == "" {
		size = "M"
	}
	sizes := map[string]library.RoleSize{}
	for _, n := range derived.Plan.Nodes {
		sizes[n.Role] = library.RoleSize{Size: size}
	}
	out, err := compile.Compile(ctx, reg, compile.Input{
		RunID: uuid.New(), Tenant: "test", Database: dspec, Plan: derived.Plan,
		EffectiveConfigs: derived.EffectiveConfigs, Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: provider,
		ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat,
	})
	if err != nil {
		t.Fatal(err)
	}
	return out, compile.EffectiveWithTuned(derived.EffectiveConfigs, out.Tuned)
}

// mustContain fails on every line the text lacks.
func mustContain(t *testing.T, what, text string, lines ...string) {
	t.Helper()
	for _, l := range lines {
		if !strings.Contains(text, l) {
			t.Errorf("%s: missing %q", what, l)
		}
	}
}

// hugePagesOf is vm.nr_hugepages of a machine's sysctl drop-in.
func hugePagesOf(out compile.Output, machine string) string {
	for _, h := range out.Spec.HostPrep {
		if h.Machine != machine || h.Kind != spec.HostPrepSysctl {
			continue
		}
		for _, l := range strings.Split(h.Content, "\n") {
			if v, ok := strings.CutPrefix(l, "vm.nr_hugepages = "); ok {
				return v
			}
		}
	}
	return ""
}

func containerOf(out compile.Output, name string) spec.Container {
	for _, c := range out.Spec.Containers {
		if c.Name == name {
			return c
		}
	}
	return spec.Container{}
}

func TestPostgresAutoTuningBySize(t *testing.T) {
	cases := []struct {
		size                                       string
		shared, cache, maint, avac, maxWAL, minWAL int
		workers, parallel, shm, conns              int
		hugePages                                  string
	}{
		// S = 4 vCPU / 16 GB / 100 GB, M = 8 / 32 / 200, L = 16 / 64 / 400 io-m3
		// (465 GB: five 93 GB allocation units).
		// max_connections: the 128-VU pool capped at 8 per vCPU, + 20, up to 50s, ≥ 100.
		{"S", 4096, 12288, 1024, 512, 4096, 1024, 8, 4, 4096, 100, "2317"},
		{"M", 8192, 24576, 2048, 1024, 8192, 2048, 8, 8, 8192, 100, "4570"},
		{"L", 16384, 49152, 2048, 1024, 19046, 4761, 16, 16, 8192, 150, "9076"},
	}
	for _, tc := range cases {
		t.Run(tc.size, func(t *testing.T) {
			out, eff := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", size: tc.size})
			conf := fileOf(out, "db-1-postgres", "/etc/stroppy/postgresql.conf")
			mustContain(t, "postgresql.conf", conf,
				"shared_buffers = "+itoa(tc.shared)+"MB\n",
				"effective_cache_size = "+itoa(tc.cache)+"MB\n",
				"work_mem = 16MB\n",
				"maintenance_work_mem = "+itoa(tc.maint)+"MB\n",
				"autovacuum_work_mem = "+itoa(tc.avac)+"MB\n",
				"huge_pages = try\n",
				"max_wal_size = "+itoa(tc.maxWAL)+"MB\n",
				"min_wal_size = "+itoa(tc.minWAL)+"MB\n",
				"checkpoint_timeout = 900s\n",
				"checkpoint_completion_target = 0.9\n",
				"wal_compression = lz4\n",
				"random_page_cost = 1.1\n",
				"effective_io_concurrency = 200\n",
				"maintenance_io_concurrency = 200\n",
				"jit = off\n",
				"max_connections = "+itoa(tc.conns)+"\n",
				"max_worker_processes = "+itoa(tc.workers)+"\n",
				"max_parallel_workers = "+itoa(tc.parallel)+"\n",
				"max_parallel_workers_per_gather = 0\n",
				"autovacuum_naptime = 10s\n",
				"autovacuum_vacuum_scale_factor = 0.05\n",
				"autovacuum_analyze_scale_factor = 0.02\n",
				"autovacuum_max_workers = 5\n",
				"autovacuum_vacuum_cost_limit = 2000\n",
			)
			got := effectiveOf(t, eff, "db", "cfg.postgresql.conf@17")
			for k, want := range map[string]float64{"shared_buffers": float64(tc.shared), "max_wal_size": float64(tc.maxWAL), "max_connections": float64(tc.conns)} {
				if got[k] != want {
					t.Errorf("effective %s = %v, want %v", k, got[k], want)
				}
			}
			if got := hugePagesOf(out, "db-1"); got != tc.hugePages {
				t.Errorf("vm.nr_hugepages = %s, want %s", got, tc.hugePages)
			}
			if got := containerOf(out, "db-1-postgres").ShmSizeMB; got != int64(tc.shm) {
				t.Errorf("shm = %d MB, want %d", got, tc.shm)
			}
		})
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// The server takes the client's pool, not every virtual user: 400 VUs on a
// 32-vCPU primary share 8 × 32 = 256 connections.
func TestPostgresAutoTuningConnectionsFollowThePool(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", size: "XL", vus: 400, params: map[string]any{"replicas": 1}})
	for _, name := range []string{"db-1-postgres", "db-replica-1-postgres"} {
		mustContain(t, name, fileOf(out, name, "/etc/stroppy/postgresql.conf"), "max_connections = 300\n", "wal_keep_size = 2048MB\n")
	}
	if pool := out.Spec.Workload.Driver["pool"].(map[string]any)["maxConns"]; pool != 256 {
		t.Errorf("client pool = %v, want 256", pool)
	}
}

func TestPostgres18IOConcurrency(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "18"})
	mustContain(t, "postgresql.conf", fileOf(out, "db-1-postgres", "/etc/stroppy/postgresql.conf"),
		"effective_io_concurrency = 64\n", "maintenance_io_concurrency = 64\n")
}

func TestPostgresExplicitKeyWinsOverAuto(t *testing.T) {
	configs := map[string]map[string]json.RawMessage{
		"db":         {"cfg.postgresql.conf@17": json.RawMessage(`{"shared_buffers":1024,"max_wal_size":1024,"max_connections":500}`)},
		"db-replica": {},
	}
	out, eff := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", configs: configs, params: map[string]any{"replicas": 1}})
	mustContain(t, "primary", fileOf(out, "db-1-postgres", "/etc/stroppy/postgresql.conf"),
		"shared_buffers = 1024MB\n", "max_wal_size = 1024MB\n", "min_wal_size = 256MB\n", "max_connections = 500\n", "work_mem = 16MB\n")
	// A standby never runs below the primary's counters.
	mustContain(t, "replica", fileOf(out, "db-replica-1-postgres", "/etc/stroppy/postgresql.conf"),
		"shared_buffers = 8192MB\n", "max_connections = 500\n")
	if got := hugePagesOf(out, "db-1"); got != "628" {
		t.Errorf("huge pages follow the explicit shared_buffers: %s", got)
	}
	if v := out.Tuned["db"]["cfg.postgresql.conf@17"]["shared_buffers"]; v != nil {
		t.Errorf("explicit shared_buffers recorded as tuned: %v", v)
	}
	if got := effectiveOf(t, eff, "db", "cfg.postgresql.conf@17")["shared_buffers"]; got != float64(1024) {
		t.Errorf("effective shared_buffers = %v", got)
	}
}

func TestPostgresStockTuningUnchanged(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", params: map[string]any{"tuning": "stock", "pgbouncer": true, "haproxy": 1}})
	conf := fileOf(out, "db-1-postgres", "/etc/stroppy/postgresql.conf")
	mustContain(t, "postgresql.conf", conf, "shared_buffers = 128MB\n", "max_connections = 100\n", "max_wal_size = 1024MB\n", "jit = on\n", "wal_compression = off\n")
	for _, absent := range []string{"autovacuum_work_mem", "maintenance_io_concurrency", "max_prepared_transactions", "synchronous_standby_names"} {
		if strings.Contains(conf, absent) {
			t.Errorf("stock renders %s", absent)
		}
	}
	ini := fileOf(out, "proxy-1-pgbouncer", "/etc/pgbouncer/pgbouncer.ini")
	mustContain(t, "pgbouncer.ini", ini, "max_client_conn = 100\n", "default_pool_size = 20\n")
	if strings.Contains(ini, "max_prepared_statements") {
		t.Error("stock renders max_prepared_statements")
	}
	if len(out.Tuned) != 0 {
		t.Errorf("stock recorded tuned values: %v", out.Tuned)
	}
	if containerOf(out, "db-1-postgres").ShmSizeMB != 0 {
		t.Error("stock sized /dev/shm")
	}
}

func TestPostgresSyncReplicasWithoutPatroni(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", params: map[string]any{"replicas": 2, "sync_replicas": 1}})
	mustContain(t, "primary", fileOf(out, "db-1-postgres", "/etc/stroppy/postgresql.conf"),
		`synchronous_standby_names = 'ANY 1 ("db-replica-1", "db-replica-2")'`)
	for _, m := range []string{"db-replica-1", "db-replica-2"} {
		mustContain(t, m, strings.Join(containerOf(out, m+"-postgres").Cmd, " "), "-d 'application_name="+m+"'", "-R -X stream")
	}
	if strings.Contains(fileOf(out, "db-replica-1-postgres", "/etc/stroppy/postgresql.conf"), "synchronous_standby_names") {
		t.Error("a replica names synchronous standbys")
	}
}

func TestPatroniDCSParameters(t *testing.T) {
	dcsOf := func(out compile.Output) map[string]any {
		t.Helper()
		for _, c := range out.Spec.Containers {
			if c.Machine != "db-replica-2" {
				continue
			}
			for _, f := range c.Files {
				if f.Path != "/etc/stroppy/patroni.yml" {
					continue
				}
				var doc struct {
					Bootstrap struct {
						DCS struct {
							PostgreSQL struct {
								Parameters map[string]any `yaml:"parameters"`
							} `yaml:"postgresql"`
						} `yaml:"dcs"`
					} `yaml:"bootstrap"`
				}
				if err := yaml.Unmarshal([]byte(f.Content), &doc); err != nil {
					t.Fatal(err)
				}
				return doc.Bootstrap.DCS.PostgreSQL.Parameters
			}
		}
		t.Fatal("no patroni.yml on db-replica-2")
		return nil
	}
	params := map[string]any{"ha": "patroni", "replicas": 2, "sync_replicas": 1, "etcd_nodes": 3, "haproxy": 1}

	out, eff := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", params: params})
	want := map[string]any{"max_connections": "100", "max_worker_processes": "8", "max_locks_per_transaction": "64", "max_wal_senders": "10", "max_replication_slots": "10", "wal_keep_size": "2048MB"}
	got := dcsOf(out)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("dcs %s = %v, want %v", k, got[k], v)
		}
	}
	if _, ok := effectiveOf(t, eff, "db", "cfg.patroni.yml@4")["postgresql_parameters"]; !ok {
		t.Error("DCS parameters missing from the effective configs")
	}
	// Memory stays per node, in custom_conf.
	mustContain(t, "custom_conf", fileOf(out, "db-1-postgres", "/etc/stroppy/postgresql.conf"), "shared_buffers = 8192MB\n")
	if containerOf(out, "db-1-postgres").ShmSizeMB != 8192 {
		t.Error("Patroni member without /dev/shm")
	}

	explicit := map[string]map[string]json.RawMessage{"db": {"cfg.postgresql.conf@17": json.RawMessage(`{"max_connections":300}`)}}
	out, _ = compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", params: params, configs: explicit})
	if got := dcsOf(out)["max_connections"]; got != "300" {
		t.Errorf("explicit max_connections lost in the DCS: %v", got)
	}

	stock := map[string]any{"tuning": "stock"}
	for k, v := range params {
		stock[k] = v
	}
	out, _ = compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", params: stock, configs: explicit})
	if got := dcsOf(out); len(got) != 1 || got["max_connections"] != "300" {
		t.Errorf("stock DCS parameters = %v, want only the explicit max_connections", got)
	}
}

func TestPgBouncerAutoLimits(t *testing.T) {
	for _, tc := range []struct {
		vus     int
		clients string
	}{{128, "1000"}, {400, "1000"}} {
		out, _ := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", vus: tc.vus, params: map[string]any{"pgbouncer": true, "haproxy": 1}})
		ini := fileOf(out, "proxy-1-pgbouncer", "/etc/pgbouncer/pgbouncer.ini")
		mustContain(t, "pgbouncer.ini", ini,
			"max_client_conn = "+tc.clients+"\n", "default_pool_size = 32\n", "min_pool_size = 32\n",
			"reserve_pool_size = 5\n", "max_db_connections = 37\n", "max_prepared_statements = 200\n")
		if containerOf(out, "proxy-1-pgbouncer").Ulimits["nofile"] != 1048576 {
			t.Error("pgbouncer without a file limit")
		}
	}
}

func TestOrioleDBAutoTuning(t *testing.T) {
	out, eff := compileOriole(t, tuneCase{version: "beta19-pg17", params: map[string]any{"replicas": 1}})
	primary := fileOf(out, "db-1-orioledb", "/etc/stroppy/postgresql.conf")
	mustContain(t, "primary", primary,
		"shared_buffers = 256MB\n", "orioledb.main_buffers = 9728MB\n", "orioledb.undo_buffers = 608MB\n",
		"orioledb.xid_buffers = 8MB\n", "orioledb.temp_buffers = 8MB\n", "orioledb.bgwriter_num_workers = 2\n",
		"orioledb.checkpoint_completion_ratio = 1\n", "max_worker_processes = 16\n", "max_wal_size = 8192MB\n",
		"orioledb.recovery_pool_size = 3\n", "max_connections = 100\n")
	mustContain(t, "replica", fileOf(out, "db-replica-1-orioledb", "/etc/stroppy/postgresql.conf"),
		"orioledb.recovery_pool_size = 4\n", "orioledb.recovery_idx_pool_size = 2\n", "orioledb.recovery_queue_size = 32MB\n")
	// shared + main + undo, 10% over, in 2MB pages.
	if got := hugePagesOf(out, "db-1"); got != "5890" {
		t.Errorf("vm.nr_hugepages = %s", got)
	}
	if got := effectiveOf(t, eff, "db", "cfg.orioledb.postgresql.conf@17")["orioledb_main_buffers"]; got != float64(9728) {
		t.Errorf("effective main_buffers = %v", got)
	}

	out, _ = compileOriole(t, tuneCase{version: "beta19-pg17", params: map[string]any{"main_buffers_mb": 4096}})
	mustContain(t, "fixed", fileOf(out, "db-1-orioledb", "/etc/stroppy/postgresql.conf"), "orioledb.main_buffers = 4096MB\n", "orioledb.undo_buffers = 256MB\n")

	out, _ = compileOriole(t, tuneCase{version: "beta19-pg17", params: map[string]any{"default_table_access_method": false}})
	mixed := fileOf(out, "db-1-orioledb", "/etc/stroppy/postgresql.conf")
	mustContain(t, "mixed", mixed, "shared_buffers = 4096MB\n", "orioledb.main_buffers = 4096MB\n", "orioledb.checkpoint_completion_ratio = 0.5\n")

	out, _ = compileOriole(t, tuneCase{version: "beta19-pg17", params: map[string]any{"tuning": "stock"}})
	stock := fileOf(out, "db-1-orioledb", "/etc/stroppy/postgresql.conf")
	mustContain(t, "stock", stock, "shared_buffers = 128MB\n", "orioledb.main_buffers = 64MB\n", "orioledb.temp_buffers = 64MB\n")
	if strings.Contains(stock, "orioledb.undo_buffers") {
		t.Error("stock sets the undo ring")
	}
}
