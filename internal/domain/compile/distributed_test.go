package compile_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

func TestDistributedCatalogRuntimeContracts(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	for _, kind := range []catalog.DatabaseKind{catalog.Picodata, catalog.Cockroach, catalog.YDB} {
		db, _ := cat.Database(kind)
		for _, version := range db.Versions {
			for _, topology := range db.Topologies {
				t.Run(string(kind)+"/"+version.Version+"/"+topology.ID, func(t *testing.T) {
					params := map[string]any{}
					for k, v := range topology.Params {
						params[k] = v
					}
					params["version"] = version.Version
					raw, _ := json.Marshal(params)
					database, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: kind, Version: version.Version, Params: raw})
					if err != nil {
						t.Fatal(err)
					}
					workload, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: db.Protocols[0], Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)}})
					if err != nil {
						t.Fatal(err)
					}
					sizes := map[string]library.RoleSize{}
					for _, node := range derived.Plan.Nodes {
						sizes[node.Role] = library.RoleSize{Size: "S"}
					}
					out, err := compile.Compile(ctx, reg, compile.Input{RunID: uuid.New(), Tenant: "test", Database: database, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: workload, WorkloadBaked: baked, Sizes: sizes, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat})
					if err != nil {
						t.Fatal(err)
					}
					for _, c := range out.Spec.Containers {
						switch {
						case strings.HasSuffix(c.Name, "-picodata"):
							if c.ScrapePort != 8081 {
								t.Fatalf("Picodata metrics port: %d", c.ScrapePort)
							}
							if len(c.Cmd) == 0 || c.Cmd[0] != "run" {
								t.Fatal("image entrypoint duplicated")
							}
							health := strings.Join(c.Healthcheck.Cmd, " ")
							if strings.Contains(health, "admin -j") || strings.Contains(health, "SELECT") || !strings.Contains(health, "current_state.variant") {
								t.Fatal("startup probe can query SQL before peer initialization")
							}
							for _, f := range c.Files {
								if version.Version != "26.2" && strings.Contains(f.Content, "replication_mode:") {
									t.Fatal("unsupported tier field rendered")
								}
								if version.Version == "25.3" && strings.Contains(f.Content, "pgproto:") {
									t.Fatal("wrong network config generation")
								}
							}
						case strings.HasSuffix(c.Name, "-cockroach"):
							if c.ScrapePort != 8080 {
								t.Fatalf("Cockroach metrics port: %d", c.ScrapePort)
							}
							if strings.Join(c.Entrypoint, " ") != "/cockroach/cockroach" {
								t.Fatal("upstream single-node wrapper was not bypassed")
							}
							for _, arg := range c.Cmd {
								if strings.HasPrefix(arg, "--cache=") && strings.Contains(arg, " --") {
									t.Fatal("all flags passed as one argument")
								}
							}
						case strings.HasSuffix(c.Name, "-ydb"):
							if c.Scrape != "/counters/prometheus" || c.ScrapePort != 8765 {
								t.Fatal("YDB metrics endpoint must expose every counter group")
							}
							if strings.Join(c.Entrypoint, " ") != "/ydbd" || c.Cmd[0] != "server" {
								t.Fatal("YDB individual node command lost")
							}
							if c.Role == "db-compute" {
								if !strings.Contains(strings.Join(c.Healthcheck.Cmd, " "), "scheme ls /Root/stroppy") {
									t.Fatal("readiness must reach SchemeShard")
								}
								for _, arg := range c.Cmd {
									if arg == "--node" {
										t.Fatal("static node conflicts with tenant")
									}
								}
								if len(c.Mounts) != 1 || c.Mounts[0].Source != "/data/ydb" {
									t.Fatal("compute spilling must use attached data disk")
								}
							}
							for _, f := range c.Files {
								if !strings.Contains(f.Content, "type: SSD") || strings.Contains(f.Content, "interconnect_config:\n  start_tcp: true\n  port:") {
									t.Fatal("invalid YDB YAML")
								}
								var conf map[string]any
								if err := yaml.Unmarshal([]byte(f.Content), &conf); err != nil {
									t.Fatal(err)
								}
								erasure := "none"
								if topology.ID == "mirror-3-dc" {
									erasure = topology.ID
								}
								if conf["static_erasure"] != erasure {
									t.Fatalf("static erasure: %v", conf["static_erasure"])
								}
								profile, ok := conf["channel_profile_config"].(map[string]any)
								if !ok {
									t.Fatal("system tablet channel profile missing")
								}
								profiles := profile["profile"].([]any)
								if len(profiles) != 1 {
									t.Fatal("expected exactly profile zero")
								}
								p := profiles[0].(map[string]any)
								channels := p["channel"].([]any)
								if p["profile_id"] != 0 || len(channels) != 3 {
									t.Fatal("invalid system tablet channels")
								}
								for _, v := range channels {
									ch := v.(map[string]any)
									if ch["erasure_species"] != erasure || ch["pdisk_category"] != 1 || ch["storage_pool_kind"] != "ssd" {
										t.Fatalf("channel placement differs from topology: %v", ch)
									}
								}
							}
						case strings.HasSuffix(c.Name, "-init"):
							if strings.Join(c.Entrypoint, " ") != "/bin/sh" || c.Cmd[0] != "-c" {
								t.Fatal("bootstrap shell not selected")
							}
							if kind == catalog.Cockroach && topology.ID == "three-node" && !strings.Contains(strings.Join(c.Cmd, " "), "--cluster-name") {
								t.Fatal("cluster init identity missing")
							}
						}
					}
					if kind == catalog.YDB && topology.ID == "mirror-3-dc" {
						counts := map[string]int{}
						for _, machine := range out.Spec.Machines {
							if machine.Role == "db" {
								counts[machine.Location]++
								if machine.Disks[0].GB < 140 {
									t.Fatal("YDB pdisks exceed attached disk capacity")
								}
							}
						}
						if counts["ru-central1-a"] != 3 || counts["ru-central1-b"] != 3 || counts["ru-central1-d"] != 3 {
							t.Fatalf("physical placement: %v", counts)
						}
					}
					if kind == catalog.YDB && len(out.Spec.Scrapes) != 0 {
						t.Fatalf("role scrapes duplicated per node: %d", len(out.Spec.Scrapes))
					}
				})
			}
		}
	}
}
