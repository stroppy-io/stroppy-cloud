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
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The catalog's example of a custom image — the official postgres one —
// compiles into that image on each db machine, its data on the data disk,
// its own ready check, and a URL built from the login it was told to make.
func TestCompileCustomImage(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil)
	kind, _ := dbkind.Lookup(catalog.Custom)
	params := kind.Topologies[0].Params
	params["nodes"] = 2
	params["metrics_port"] = 9187
	raw, _ := json.Marshal(params)
	database, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: catalog.Custom, Version: "custom", Params: raw})
	if err != nil {
		t.Fatal(err)
	}
	if shape := derived.Plan.Shape(); shape.Nodes[0].Role != "db" || shape.Nodes[0].Count != 2 || len(database.Topology.Nodes) != len(shape.Nodes) {
		t.Fatalf("shape %+v / %+v", shape, database.Topology)
	}
	workload, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: catalog.ProtoPg, Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := cat.Provider("yandex")
	sizes := map[string]library.RoleSize{"runner": {Size: "S"}, "db": {Size: "S"}}
	out, err := compile.Compile(ctx, reg, compile.Input{RunID: uuid.New(), Tenant: "test", Database: database, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: workload, WorkloadBaked: baked, Sizes: sizes, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Spec.Workload.URL; got != "postgresql://stroppy:stroppy@${ip:role:db}:5432/stroppy?sslmode=disable" {
		t.Fatalf("url %s", got)
	}
	var dbs []spec.Container
	for _, c := range out.Spec.Containers {
		if c.Kind == spec.ContainerKindDatabase {
			dbs = append(dbs, c)
		}
	}
	if len(dbs) != 2 {
		t.Fatalf("want a database container per machine, got %d", len(dbs))
	}
	c := dbs[0]
	if c.Image != "postgres:17" || c.Env["POSTGRES_DB"] != "stroppy" || len(c.Ports) != 1 || c.Ports[0].Host != 5432 {
		t.Fatalf("container %+v", c)
	}
	if c.Healthcheck == nil || c.Healthcheck.Cmd[1] != "pg_isready -U stroppy -d stroppy" {
		t.Fatalf("healthcheck %+v", c.Healthcheck)
	}
	if len(c.Mounts) != 1 || c.Mounts[0].Target != "/var/lib/postgresql/data" {
		t.Fatalf("mounts %+v", c.Mounts)
	}
	// The image's endpoint is scraped, and a pg image gets postgres_exporter
	// logged in with the tenant's user.
	if c.Scrape != "/metrics" || c.ScrapePort != 9187 {
		t.Fatalf("scrape %q:%d", c.Scrape, c.ScrapePort)
	}
	exporters := 0
	for _, e := range out.Spec.Containers {
		if e.Kind == spec.ContainerKindExporter && strings.HasSuffix(e.Name, "-postgres-exporter") &&
			e.Env["DATA_SOURCE_NAME"] == "postgresql://stroppy:stroppy@127.0.0.1:5432/stroppy?sslmode=disable" {
			exporters++
		}
	}
	if exporters != 2 {
		t.Fatalf("postgres exporters %d, want one per db machine", exporters)
	}
}
