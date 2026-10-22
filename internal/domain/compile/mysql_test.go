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
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

func TestCompileMySQLFamilyConfigVersions(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	for _, kind := range []catalog.DatabaseKind{catalog.MySQL, catalog.MariaDB} {
		db, _ := cat.Database(kind)
		for _, version := range db.Versions {
			t.Run(string(kind)+"/"+version.Version, func(t *testing.T) {
				prefix := "cfg.my.cnf@"
				params := map[string]any{"version": version.Version, "replicas": 2, "replication": "semi_sync", "semi_sync_wait_for_slave_count": 2}
				if kind == catalog.MariaDB {
					prefix = "cfg.mariadb.cnf@"
					params = map[string]any{"version": version.Version}
				}
				raw, _ := json.Marshal(params)
				schemaVersion := strings.TrimSuffix(version.Version, ".0")
				input := library.DatabaseSpec{
					Kind: kind, Version: version.Version, Params: raw,
					Configs: map[string]map[string]json.RawMessage{"db": {prefix + schemaVersion: json.RawMessage(`{"max_connections":222}`)}},
				}
				dspec, derived, err := lib.DeriveDatabase(ctx, input)
				if err != nil {
					t.Fatal(err)
				}
				wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{
					StroppyVersion: "6.0.0", Protocol: catalog.ProtoMySQL,
					Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)},
				})
				if err != nil {
					t.Fatal(err)
				}
				sizes := map[string]library.RoleSize{}
				for _, n := range derived.Plan.Nodes {
					sizes[n.Role] = library.RoleSize{Size: "S"}
				}
				out, err := compile.Compile(ctx, reg, compile.Input{
					RunID: uuid.New(), Tenant: "test", Database: dspec, Plan: derived.Plan,
					EffectiveConfigs: derived.EffectiveConfigs, Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: provider,
					ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat,
				})
				if err != nil {
					t.Fatal(err)
				}
				configs := 0
				for _, c := range out.Spec.Containers {
					for _, f := range c.Files {
						if f.Path != "/etc/mysql/conf.d/zz-stroppy.cnf" {
							continue
						}
						configs++
						if !strings.Contains(strings.SplitN(f.Content, "\n", 2)[0], version.Version) {
							t.Error("configuration does not match selected database version")
						}
						if c.Role == "db" && !strings.Contains(f.Content, "max_connections = 222") {
							t.Error("user override lost")
						}
						if kind == catalog.MariaDB && version.Version == "10.11" &&
							(!strings.Contains(f.Content, "\ntransaction-isolation = REPEATABLE-READ\n") || strings.Contains(f.Content, "\ntx_isolation =")) {
							t.Error("MariaDB 10.11 requires the startup option, not the SQL variable name")
						}
						if kind == catalog.MySQL {
							if strings.Contains(f.Content, "\nrpl_semi_sync_") {
								t.Error("plugin options must allow first datadir initialization")
							}
							if version.Version == "8.0" && strings.Contains(f.Content, "\nmysql_native_password =") {
								t.Error("MySQL 8.4-only option on 8.0")
							}
							if c.Role == "db" && !strings.Contains(f.Content, "loose-rpl_semi_sync_source_wait_for_replica_count = 2") {
								t.Error("requested two acknowledgements were not configured")
							}
							if c.Role == "db-replica" && !strings.Contains(f.Content, "\nread_only = ON\n") {
								t.Error("replica read_only must survive the entrypoint restart")
							}
							if c.Role == "db-replica" && !strings.Contains(f.Content, "loose-rpl_semi_sync_replica_enabled = ON") {
								t.Error("semisynchronous replica was not enabled")
							}
						}
					}
				}
				if configs == 0 {
					t.Fatal("database configuration missing")
				}
				input.Configs = map[string]map[string]json.RawMessage{"db": {prefix + "99.0": json.RawMessage(`{"max_connections":222}`)}}
				if _, _, err := lib.DeriveDatabase(ctx, input); err == nil {
					t.Error("unrelated configuration version silently accepted")
				}
			})
		}
	}
}

// The buffer pool follows the database's buffer_pool mode; a config that
// sets it explicitly wins over every mode.
func TestCompileInnoDBBufferPool(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	compileConf := func(t *testing.T, kind catalog.DatabaseKind, params map[string]any, configs map[string]map[string]json.RawMessage) (string, int) {
		t.Helper()
		raw, _ := json.Marshal(params)
		dspec, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: kind, Version: params["version"].(string), Params: raw, Configs: configs})
		if err != nil {
			t.Fatal(err)
		}
		wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{
			StroppyVersion: "6.0.0", Protocol: catalog.ProtoMySQL,
			Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)},
		})
		if err != nil {
			t.Fatal(err)
		}
		sizes := map[string]library.RoleSize{}
		for _, n := range derived.Plan.Nodes {
			sizes[n.Role] = library.RoleSize{Size: "M"}
		}
		out, err := compile.Compile(ctx, reg, compile.Input{
			RunID: uuid.New(), Tenant: "test", Database: dspec, Plan: derived.Plan,
			EffectiveConfigs: derived.EffectiveConfigs, Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: provider,
			ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat,
		})
		if err != nil {
			t.Fatal(err)
		}
		memory := 0
		for _, m := range out.Spec.Machines {
			if m.Role == "db" {
				memory = m.MemoryGB
			}
		}
		for _, c := range out.Spec.Containers {
			for _, f := range c.Files {
				if c.Role == "db" && f.Path == "/etc/mysql/conf.d/zz-stroppy.cnf" {
					return f.Content, memory
				}
			}
		}
		t.Fatal("no my.cnf for the db role")
		return "", 0
	}
	t.Run("auto takes 70% of the machine", func(t *testing.T) {
		conf, gb := compileConf(t, catalog.MariaDB, map[string]any{"version": "11.4"}, nil)
		want := fmt.Sprintf("innodb_buffer_pool_size = %dM", int(float64(gb*1024)*0.7))
		// 200 GB network-ssd: 7000 write IOPS, 105 MB/s.
		if !strings.Contains(conf, want) || !strings.Contains(conf, "innodb_io_capacity = 2600\n") {
			t.Errorf("want %q and a disk-class io capacity in\n%s", want, conf)
		}
	})
	t.Run("fixed takes buffer_pool_mb", func(t *testing.T) {
		conf, _ := compileConf(t, catalog.MySQL, map[string]any{"version": "8.4", "buffer_pool": "fixed", "buffer_pool_mb": 2048}, nil)
		if !strings.Contains(conf, "innodb_buffer_pool_size = 2048M") {
			t.Errorf("fixed size lost:\n%s", conf)
		}
	})
	t.Run("stock keeps the server default", func(t *testing.T) {
		conf, _ := compileConf(t, catalog.MariaDB, map[string]any{"version": "11.4", "buffer_pool": "stock"}, nil)
		if !strings.Contains(conf, "innodb_buffer_pool_size = 128M") || !strings.Contains(conf, "innodb_io_capacity = 200") {
			t.Errorf("stock changed the defaults:\n%s", conf)
		}
	})
	t.Run("an explicit config wins", func(t *testing.T) {
		conf, _ := compileConf(t, catalog.MySQL, map[string]any{"version": "8.4"},
			map[string]map[string]json.RawMessage{"db": {"cfg.my.cnf@8.4": json.RawMessage(`{"innodb_buffer_pool_size":1024}`)}})
		if !strings.Contains(conf, "innodb_buffer_pool_size = 1024M") {
			t.Errorf("explicit size overridden:\n%s", conf)
		}
	})
}
