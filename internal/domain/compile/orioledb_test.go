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

func TestCompileOrioleDBCatalog(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	db, _ := cat.Database(catalog.OrioleDB)
	for _, version := range db.Versions {
		for _, replicas := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/replicas-%d", version.Version, replicas), func(t *testing.T) {
				raw, _ := json.Marshal(map[string]any{"image_tag": version.Version, "replicas": replicas})
				major := strings.Split(version.Version, "pg")[1]
				conf := json.RawMessage(`{"pg_stat_statements_max":10000,"pg_stat_statements_track":"all"}`)
				input := library.DatabaseSpec{Kind: catalog.OrioleDB, Version: version.Version, Params: raw,
					Configs: map[string]map[string]json.RawMessage{"db": {"cfg.orioledb.postgresql.conf@" + major: conf}, "db-replica": {"cfg.orioledb.postgresql.conf@" + major: conf}}}
				if replicas == 0 {
					delete(input.Configs, "db-replica")
				}
				dspec, derived, err := lib.DeriveDatabase(ctx, input)
				if err != nil {
					t.Fatal(err)
				}
				wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: catalog.ProtoPg,
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
				count := 0
				for _, c := range out.Spec.Containers {
					if !strings.HasSuffix(c.Name, "-orioledb") {
						continue
					}
					count++
					cmd := strings.Join(c.Cmd, " ")
					if strings.Contains(cmd, "-D /etc/postgresql") {
						t.Error("configuration directory used as PGDATA")
					}
					if c.Env["PGDATA"] != "/var/lib/postgresql/data" || len(c.Mounts) != 1 || c.Mounts[0].Target != c.Env["PGDATA"] {
						t.Error("data mount and PGDATA disagree")
					}
					found := false
					for _, f := range c.Files {
						if f.Path != "/etc/stroppy/postgresql.conf" {
							continue
						}
						found = true
						for _, line := range []string{"shared_preload_libraries = 'pg_stat_statements,orioledb'", "pg_stat_statements.max = 10000", "pg_stat_statements.track = all"} {
							if !strings.Contains(f.Content, line) {
								t.Errorf("missing %s", line)
							}
						}
						if strings.Contains(f.Content, "orioledb.undo_buffers =") {
							t.Error("automatic undo size overridden")
						}
					}
					if !found {
						t.Error("rendered config missing")
					}
					if c.Role == "db-replica" && (!strings.Contains(cmd, "pg_basebackup") || !strings.Contains(cmd, "-R -X stream") || len(c.DependsOn) != 1) {
						t.Error("physical replication bootstrap missing")
					}
				}
				if count != replicas+1 {
					t.Fatalf("got %d database containers", count)
				}
			})
		}
	}
}
