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
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func runtimeInput(t *testing.T) (*schemas.Registry, compile.Input) {
	t.Helper()
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	db, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: catalog.Postgres, Version: "17", Params: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	wl, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: catalog.ProtoPg, Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"tpcc/tx","scale_factor":1},"run":{"duration":"1m","vus":1}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := cat.Provider("yandex")
	sizes := map[string]library.RoleSize{}
	for _, n := range derived.Plan.Nodes {
		sizes[n.Role] = library.RoleSize{Size: "S"}
	}
	return reg, compile.Input{RunID: uuid.New(), Tenant: "acme", Database: db, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: wl, WorkloadBaked: baked, Sizes: sizes, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "provider-x", ProviderConfigName: "t-acme", Catalog: cat}
}

func TestCompileRuntimeConfigAndHardware(t *testing.T) {
	reg, in := runtimeInput(t)
	base, err := compile.Compile(context.Background(), reg, in)
	if err != nil {
		t.Fatal(err)
	}
	var db spec.Container
	for _, c := range base.Spec.Containers {
		if c.Role == "db" && !strings.Contains(c.Name, "exporter") && len(c.Files) > 0 {
			db = c
			break
		}
	}
	if db.Name == "" {
		t.Fatal("no generated database config")
	}
	file := db.Files[0].Path
	runtime, _ := json.Marshal(map[string]any{"containers": []any{map[string]any{"name": db.Name, "set": map[string]any{"files": []any{map[string]any{"path": file, "content": "# exact user config\nshared_buffers = 256MB\n"}}, "env": map[string]string{"USER_OPTION": "kept"}}}}})
	in.Database.Runtime = runtime
	in.Sizes["db"] = library.RoleSize{Size: "S", Machine: json.RawMessage(`{"cpu":8,"memory_gb":32,"boot_disk":{"gb":80,"type":"network-ssd"},"disks":[{"name":"data","gb":100,"type":"network-ssd","mount":"/data"},{"name":"wal","gb":93,"type":"network-ssd-nonreplicated","mount":"/wal"}]}`)}
	in.Execution = json.RawMessage(`{"machines":{"db-1":{"cpu":16,"memory_gb":64,"preemptible":false}}}`)
	out, err := compile.Compile(context.Background(), reg, in)
	if err != nil {
		t.Fatal(err)
	}
	m := out.Spec.MachineByName()["db-1"]
	if m.CPU != 16 || m.MemoryGB != 64 || m.BootDisk.GB != 80 || len(m.Disks) != 2 || m.Preemptible == nil || *m.Preemptible {
		t.Fatalf("final hardware %+v", m)
	}
	matched := false
	for _, c := range out.Spec.Containers {
		if c.Name == db.Name {
			for _, f := range c.Files {
				if f.Path == file {
					matched = f.Content == "# exact user config\nshared_buffers = 256MB\n"
				}
			}
		}
	}
	if !matched {
		t.Fatal("raw file override lost")
	}
	wal := false
	for _, h := range out.Spec.HostPrep {
		if h.Machine == "db-1" && strings.Contains(h.Content, "virtio-wal") && strings.Contains(h.Content, "target='/wal'") {
			wal = true
		}
	}
	if !wal {
		t.Fatal("secondary WAL disk will not be mounted on selected machine")
	}
	if _, err := spec.NormalizeRun(out.Spec); err != nil {
		t.Fatal(err)
	}
	for _, m := range out.Machines {
		if m.Name == "db-1" && m.CPU != 16 {
			t.Fatal("snapshot lost final override")
		}
	}
}

func TestCompileRejectsImpossibleExplicitDisk(t *testing.T) {
	reg, in := runtimeInput(t)
	in.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"huge","workload":{"script":"tpcc/tx","scale_factor":100000},"run":{"vus":1,"duration":"1m"}}`)}
	var baked map[string]any
	if err := json.Unmarshal(in.WorkloadBaked, &baked); err != nil {
		t.Fatal(err)
	}
	baked["segments"] = in.Workload.Segments
	in.WorkloadBaked, _ = json.Marshal(baked)
	in.Sizes["db"] = library.RoleSize{Size: "S", DiskGB: 10}
	if _, err := compile.Compile(context.Background(), reg, in); err == nil || !strings.Contains(err.Error(), "storage budget") {
		t.Fatalf("expected capacity rejection: %v", err)
	}
	in.Sizes["db"] = library.RoleSize{Size: "S"}
	out, err := compile.Compile(context.Background(), reg, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Spec.MachineByName()["db-1"].Disks[0].GB < 30000 {
		t.Fatal("default did not grow with workload")
	}
}
