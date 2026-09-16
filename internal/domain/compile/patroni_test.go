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

func TestPatroniVersions(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	for _, version := range []string{"15", "16", "17", "18"} {
		t.Run(version, func(t *testing.T) {
			params, _ := json.Marshal(map[string]any{"version": version, "ha": "patroni", "replicas": 2, "sync_replicas": 1, "etcd_nodes": 3, "haproxy": 1})
			db, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: catalog.Postgres, Version: version, Params: params, Configs: map[string]map[string]json.RawMessage{
				"db": {"cfg.patroni.yml@4": json.RawMessage(`{"ttl":60}`), "cfg.postgresql.conf@" + version: json.RawMessage(`{"shared_buffers":256}`)},
			}})
			if err != nil {
				t.Fatal(err)
			}
			w, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: catalog.ProtoPg, Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)}})
			if err != nil {
				t.Fatal(err)
			}
			sizes := map[string]library.RoleSize{}
			for _, n := range derived.Plan.Nodes {
				sizes[n.Role] = library.RoleSize{Size: "S"}
			}
			out, err := compile.Compile(ctx, reg, compile.Input{RunID: uuid.New(), Tenant: "test", Database: db, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: w, WorkloadBaked: baked, Sizes: sizes, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", ProviderConfigName: "t-test", Catalog: cat})
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(out.Spec)
			if _, err := reg.Bake(ctx, "spec.run@1", raw); err != nil {
				t.Fatal(err)
			}
			patroni, etcd := 0, 0
			scrapes := map[string]string{}
			for _, scrape := range out.Spec.Scrapes {
				scrapes[scrape.Job] = scrape.URL
			}
			for _, ct := range out.Spec.Containers {
				if strings.Contains(ct.Image, "/patroni:") && scrapes[ct.Name] != "http://127.0.0.1:8008/metrics" {
					t.Error("Patroni metrics not bound to its container")
				}
				if ct.Role == "etcd" && strings.HasSuffix(ct.Name, "-etcd") && ct.Scrape != "/metrics" {
					t.Error("etcd metrics missing")
				}

				for _, f := range ct.Files {
					switch f.Path {
					case "/etc/stroppy/patroni.yml":
						patroni++
						var doc map[string]any
						if err := yaml.Unmarshal([]byte(f.Content), &doc); err != nil {
							t.Fatal(err)
						}
						pg := doc["postgresql"].(map[string]any)
						if pg["bin_dir"] != "/usr/lib/postgresql/"+version+"/bin" || pg["custom_conf"] != "/etc/stroppy/postgresql.conf" {
							t.Fatal("wrong PostgreSQL config paths")
						}
						if _, bad := pg["pg_hba"]; bad {
							t.Fatal("file path passed as inline pg_hba list")
						}
						dcs := doc["bootstrap"].(map[string]any)["dcs"].(map[string]any)
						if dcs["synchronous_mode"] != "on" || dcs["synchronous_mode_strict"] != true {
							t.Fatal("synchronous topology lost")
						}
						if dcs["ttl"] != 60 {
							t.Fatal("Patroni override lost")
						}
						if !strings.Contains(ct.Image, "pg"+version+"-4.1.5") || len(ct.DependsOn) != 3 {
							t.Fatal("wrong image or DCS dependencies")
						}
					case "/etc/stroppy/etcd.yml":
						etcd++
						if !strings.Contains(f.Content, "etcd-3=http://") {
							t.Fatal("incomplete etcd membership")
						}
					case "/usr/local/etc/haproxy/haproxy.cfg":
						for _, want := range []string{"uri /primary", "uri /replica", "port 8008", "db-replica-2"} {
							if !strings.Contains(f.Content, want) {
								t.Errorf("HAProxy missing %s", want)
							}
						}
					}
				}
			}
			if patroni != 3 || etcd != 3 {
				t.Fatalf("members: patroni=%d etcd=%d", patroni, etcd)
			}
			flows := map[string]bool{}
			for _, f := range out.Spec.Flows {
				flows[f.FromRole+"/"+f.ToRole+"/"+f.Label] = true
			}
			for _, key := range []string{"etcd/etcd/etcd-peer", "db/db-replica/pg-streaming", "db-replica/db-replica/patroni-api", "proxy/db-replica/patroni-api"} {
				if !flows[key] {
					t.Errorf("missing flow %s", key)
				}
			}
		})
	}
}
