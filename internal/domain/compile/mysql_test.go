package compile_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

func TestCompileMySQLFamilyConfigVersions(t *testing.T) {
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
			t.Run(string(kind)+"/"+version.Version, func(t *testing.T) {
				prefix := "cfg.my.cnf@"
				params := map[string]any{"version": version.Version, "replicas": 2, "replication": "semi_sync", "semi_sync_wait_for_slave_count": 2}
				if kind == catalog.MariaDB {
					prefix = "cfg.mariadb.cnf@"
					params = map[string]any{"version": version.Version}
				}
				raw, _ := json.Marshal(params)
				schemaVersion := strings.TrimSuffix(version.Version, ".0")
				input := library.DatabaseSpec{Kind: kind, Version: version.Version, Params: raw,
					Configs: map[string]map[string]json.RawMessage{"db": {prefix + schemaVersion: json.RawMessage(`{"max_connections":222}`)}}}
				dspec, derived, err := lib.DeriveDatabase(ctx, input)
				if err != nil {
					t.Fatal(err)
				}
				wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: catalog.ProtoMySQL,
					Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)}})
				if err != nil {
					t.Fatal(err)
				}
				sizes := map[string]library.RoleSize{}
				for _, n := range derived.Plan.Nodes {
					sizes[n.Role] = library.RoleSize{Size: "S"}
				}
				out, err := compile.Compile(ctx, reg, compile.Input{RunID: uuid.New(), Tenant: "test", Database: dspec, Plan: derived.Plan,
					EffectiveConfigs: derived.EffectiveConfigs, Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: provider,
					ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1g","folder_id":"b1g","zone":"ru-central1-a"}`), CredentialsSecret: "yc", Catalog: cat})
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
