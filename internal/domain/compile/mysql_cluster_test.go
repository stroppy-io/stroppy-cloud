package compile_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

// Exercise the actual catalog, schema renderer and RunSpec validation together.
func TestCompileMySQLClusters(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	for _, kind := range []catalog.DatabaseKind{catalog.MySQL, catalog.MariaDB} {
		db, _ := cat.Database(kind)
		for _, version := range db.Versions {
			for _, topo := range db.Topologies {
				if topo.ID != "group-replication" && topo.ID != "galera" {
					continue
				}
				t.Run(string(kind)+"/"+version.Version, func(t *testing.T) {
					params := map[string]any{}
					raw, _ := json.Marshal(topo.Params)
					if err := json.Unmarshal(raw, &params); err != nil {
						t.Fatal(err)
					}
					params["version"] = version.Version
					raw, _ = json.Marshal(params)
					dspec, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: kind, Version: version.Version, Params: raw})
					if err != nil {
						t.Fatal(err)
					}
					wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: db.Protocols[0], Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)}})
					if err != nil {
						t.Fatal(err)
					}
					sizes := map[string]library.RoleSize{}
					for _, n := range derived.Plan.Nodes {
						sizes[n.Role] = library.RoleSize{Size: "S"}
					}
					runID := uuid.New()
					out, err := compile.Compile(ctx, reg, compile.Input{RunID: runID, Tenant: "test", Database: dspec, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat})
					if err != nil {
						t.Fatal(err)
					}
					members, proxies := 0, 0
					for _, ct := range out.Spec.Containers {
						var conf string
						for _, f := range ct.Files {
							conf += f.Content + "\n"
						}
						if strings.HasSuffix(ct.Name, "-mysql") || strings.HasSuffix(ct.Name, "-mariadb") {
							members++
							if !strings.Contains(conf, fmt.Sprintf("server_id = %d\n", members)) {
								t.Error("missing unique server id", ct.Name)
							}
							if len(ct.Mounts) != 1 || ct.Mounts[0].Source != "/data/mysql" {
								t.Error("missing separate data disk", ct.Name)
							}
							if members > 1 && (len(ct.DependsOn) != 1 || ct.Env["MYSQL_DATABASE"] != "") {
								t.Error("joiner must follow the previous member without local database creation", ct.Name)
							}
							if kind == catalog.MySQL {
								for _, line := range []string{"loose-group_replication_group_name = " + runID.String(), "loose-group_replication_start_on_boot = OFF", "loose-group_replication_consistency = BEFORE", "loose-group_replication_bootstrap_group = OFF", "loose-group_replication_local_address = ${ip:" + ct.Machine + "}:33061", "env -u MYSQL_PWD docker-entrypoint.sh mysqld", "SET SESSION SQL_LOG_BIN=0;"} {
									if !strings.Contains(conf, line) {
										t.Errorf("%s missing %s", ct.Name, line)
									}
								}
								if members > 1 && ct.Env["MYSQL_INITDB_SKIP_TZINFO"] != "1" {
									t.Error("joiner timezone GTIDs diverge")
								}
							} else {
								if members == 1 && !strings.Contains(conf, "GRANT SLAVE MONITOR ON *.* TO 'exporter'@'%';") {
									t.Error("MariaDB exporter cannot collect replica status")
								}
								for _, line := range []string{"wsrep_cluster_name = " + strings.ReplaceAll(runID.String(), "-", ""), "wsrep_sst_method = mariabackup", "wsrep_sync_wait = 1", "wsrep_sst_auth = root:", "--wsrep-on=OFF --wsrep-provider=none"} {
									if !strings.Contains(conf, line) {
										t.Errorf("%s missing %s", ct.Name, line)
									}
								}
							}
						}
						if strings.HasSuffix(ct.Name, "-proxysql") {
							proxies++
							if len(ct.DependsOn) != 3 {
								t.Error("proxy must await every database member")
							}
							if ct.Scrape != "/metrics" || ct.Ports[0].Container != 6070 {
								t.Error("ProxySQL metrics port is not scraped")
							}
							block := "mysql_group_replication_hostgroups"
							if kind == catalog.MariaDB {
								block = "mysql_galera_hostgroups"
								if !strings.Contains(conf, "writer_is_also_reader=2") {
									t.Error("Galera reader pool must include backup writers")
								}
							}
							if !strings.Contains(conf, block) || !strings.Contains(strings.Join(ct.Healthcheck.Cmd, " "), "/etc/stroppy/proxy-health.sh") {
								t.Error("missing cluster proxy configuration")
							}
						}
					}
					if members != 3 || proxies != 1 {
						t.Fatalf("members=%d proxies=%d", members, proxies)
					}
				})
			}
		}
	}
}

func TestClusterConsistencyOverrides(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	for _, tc := range []struct {
		kind                           catalog.DatabaseKind
		version, schema, params, field string
		value                          any
	}{
		{catalog.MySQL, "8.4", "cfg.my.cnf@8.4", `{"replication":"group","replicas":2,"proxysql":1}`, "group_replication_consistency", "EVENTUAL"},
		{catalog.MariaDB, "11.4", "cfg.mariadb.cnf@11.4", `{"replication":"galera","galera_nodes":3,"proxysql":1}`, "wsrep_sync_wait", float64(0)},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			override, err := json.Marshal(map[string]any{tc.field: tc.value})
			if err != nil {
				t.Fatal(err)
			}
			_, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: tc.kind, Version: tc.version, Params: json.RawMessage(tc.params), Configs: map[string]map[string]json.RawMessage{"db": {tc.schema: override}}})
			if err != nil {
				t.Fatal(err)
			}
			var values map[string]any
			if err := json.Unmarshal(derived.EffectiveConfigs["db"][tc.schema], &values); err != nil {
				t.Fatal(err)
			}
			if values[tc.field] != tc.value {
				t.Fatalf("explicit consistency override lost: %v", values[tc.field])
			}
		})
	}
}

func TestGaleraProxyReaderOverride(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	for _, value := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			raw := json.RawMessage(fmt.Sprintf(`{"writer_is_also_reader":%d,"read_write_split":"true"}`, value))
			dspec, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: catalog.MariaDB, Version: "11.4", Params: json.RawMessage(`{"replication":"galera","galera_nodes":3,"proxysql":1}`), Configs: map[string]map[string]json.RawMessage{"proxy": {"cfg.proxysql.cnf@2": raw}}})
			if err != nil {
				t.Fatal(err)
			}
			var cfg map[string]any
			if err := json.Unmarshal(derived.EffectiveConfigs["proxy"]["cfg.proxysql.cnf@2"], &cfg); err != nil {
				t.Fatal(err)
			}
			if cfg["writer_is_also_reader"] != float64(value) {
				t.Fatalf("explicit reader policy lost: %v", cfg)
			}
			provider, _ := cat.Provider("yandex")
			wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: "mysql", Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)}})
			if err != nil {
				t.Fatal(err)
			}
			sizes := map[string]library.RoleSize{}
			for _, node := range derived.Plan.Nodes {
				sizes[node.Role] = library.RoleSize{Size: "S"}
			}
			out, err := compile.Compile(ctx, reg, compile.Input{RunID: uuid.New(), Tenant: "test", Database: dspec, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, ct := range out.Spec.Containers {
				if !strings.HasSuffix(ct.Name, "-proxysql") {
					continue
				}
				for _, file := range ct.Files {
					if file.Path == "/etc/proxysql.cnf" {
						found = true
						if !strings.Contains(file.Content, fmt.Sprintf("writer_is_also_reader=%d", value)) {
							t.Fatalf("compiled reader policy differs from preview: %s", file.Content)
						}
					}
				}
			}
			if !found {
				t.Fatal("ProxySQL configuration missing")
			}
		})
	}
}
