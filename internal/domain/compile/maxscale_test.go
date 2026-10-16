package compile_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

// MaxScale in front of MariaDB replication (mariadbmon) and Galera
// (galeramon): the proxy container, its config over every backend, the
// accounts on the first node and the workload pointed at the listener.
func TestCompileMariaDBMaxScale(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	db, _ := cat.Database(catalog.MariaDB)
	version := db.Versions[len(db.Versions)-1].Version
	for _, tc := range []struct {
		name    string
		params  map[string]any
		module  string
		servers int
	}{
		{"replication", map[string]any{"replicas": 2, "maxscale": true}, "module=mariadbmon", 3},
		{"galera", map[string]any{"replication": "galera", "galera_nodes": 3, "maxscale": true}, "module=galeramon", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.params["version"] = version
			raw, _ := json.Marshal(tc.params)
			dspec, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: catalog.MariaDB, Version: version, Params: raw})
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
			out, err := compile.Compile(ctx, reg, compile.Input{RunID: uuid.New(), Tenant: "test", Database: dspec, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat})
			if err != nil {
				t.Fatal(err)
			}
			var proxy, init string
			for _, ct := range out.Spec.Containers {
				for _, f := range ct.Files {
					if strings.HasSuffix(ct.Name, "-maxscale") && f.Path == "/etc/maxscale.cnf" {
						proxy = f.Content
						if !strings.Contains(ct.Image, "maxscale:24.02") || len(ct.DependsOn) != tc.servers || ct.Healthcheck == nil {
							t.Errorf("maxscale container %+v", ct)
						}
					}
					if strings.Contains(f.Content, "maxscale_service") {
						init += f.Content
					}
				}
			}
			for _, want := range []string{tc.module, "router=readwritesplit", "backend_connect_timeout=", "port=4006", "password=stroppy_maxscale", "type=server", "enable_root_user=true"} {
				if !strings.Contains(proxy, want) {
					t.Errorf("maxscale.cnf lacks %q:\n%s", want, proxy)
				}
			}
			if n := strings.Count(proxy, "type=server"); n != tc.servers {
				t.Errorf("%d backends, want %d", n, tc.servers)
			}
			if strings.Count(init, "CREATE USER IF NOT EXISTS 'maxscale_service'") != 1 || !strings.Contains(init, "GRANT SELECT ON mysql.user TO 'maxscale_service'") {
				t.Errorf("accounts created once on the first node:\n%s", init)
			}
			if !strings.Contains(out.Spec.Workload.URL, ":4006") {
				t.Errorf("workload url %s", out.Spec.Workload.URL)
			}
		})
	}
}
