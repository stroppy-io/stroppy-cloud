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
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// tuneCase is one database compiled on a Yandex Cloud stand for the
// tuning tests: every role takes the same size (M = 8 vCPU / 32 GB /
// 200 GB network-ssd unless set), the workload is TPC-C at vus.
type tuneCase struct {
	kind     catalog.DatabaseKind
	version  string
	params   map[string]any
	configs  map[string]map[string]json.RawMessage
	protocol catalog.Protocol
	vus      int
	size     string
	diskGB   int
}

// compileTuned compiles a tuneCase and returns the output and the
// effective configs the run's snapshot keeps.
func compileTuned(t *testing.T, tc tuneCase) (compile.Output, map[string]map[string]json.RawMessage) {
	t.Helper()
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	params := map[string]any{"version": tc.version}
	if tc.kind == catalog.OrioleDB { // the image tag is OrioleDB's version
		params = map[string]any{"image_tag": tc.version}
	}
	for k, v := range tc.params {
		params[k] = v
	}
	raw, _ := json.Marshal(params)
	dspec, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: tc.kind, Version: tc.version, Params: raw, Configs: tc.configs})
	if err != nil {
		t.Fatal(err)
	}
	vus := tc.vus
	if vus == 0 {
		vus = 128
	}
	protocol := tc.protocol
	if protocol == "" {
		protocol = catalog.ProtoPg
	}
	segment := fmt.Sprintf(`{"name":"main","workload":{"script":"tpcc/tx","scale_factor":100},"run":{"vus":%d,"duration":"10m"}}`, vus)
	wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{
		StroppyVersion: "6.0.0", Protocol: protocol, Segments: []json.RawMessage{json.RawMessage(segment)},
	})
	if err != nil {
		t.Fatal(err)
	}
	size := tc.size
	if size == "" {
		size = "M"
	}
	sizes := map[string]library.RoleSize{}
	for _, n := range derived.Plan.Nodes {
		sizes[n.Role] = library.RoleSize{Size: size, DiskGB: tc.diskGB}
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

// fileOf is the content of a file of the first container named with suffix.
func fileOf(out compile.Output, containerSuffix, path string) string {
	for _, c := range out.Spec.Containers {
		if !strings.HasSuffix(c.Name, containerSuffix) {
			continue
		}
		for _, f := range c.Files {
			if f.Path == path {
				return f.Content
			}
		}
	}
	return ""
}

func TestAutoTuningPreparesDatabaseHosts(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17"})
	sysctl, thp := 0, 0
	for _, h := range out.Spec.HostPrep {
		switch {
		case h.Kind == spec.HostPrepSysctl && strings.Contains(h.Content, "fs.aio-max-nr = 1048576"):
			sysctl++
		case h.Kind == spec.HostPrepScript && strings.Contains(h.Content, "echo never > \"$thp/enabled\""):
			thp++
		}
	}
	if sysctl != 1 || thp != 1 {
		t.Fatalf("one database machine: %d sysctl drop-ins, %d THP scripts", sysctl, thp)
	}
	for _, c := range out.Spec.Containers {
		if c.Kind == spec.ContainerKindDatabase && c.Ulimits["nofile"] != 1048576 {
			t.Errorf("%s: nofile = %d", c.Name, c.Ulimits["nofile"])
		}
	}
	for _, h := range out.Spec.HostPrep {
		if h.Kind == spec.HostPrepScript && strings.Contains(h.Content, "mount") && !strings.Contains(h.Content, "noatime") {
			t.Error("data disk mounts without noatime")
		}
	}
}

func TestStockTuningLeavesHostsAlone(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17", params: map[string]any{"tuning": "stock"}})
	for _, h := range out.Spec.HostPrep {
		if h.Kind == spec.HostPrepSysctl {
			t.Fatal("stock tuning wrote a sysctl drop-in")
		}
	}
	for _, c := range out.Spec.Containers {
		if c.Kind == spec.ContainerKindDatabase && len(c.Ulimits) > 0 {
			t.Errorf("%s: ulimits under stock tuning", c.Name)
		}
	}
}

func TestSummaryRecordsTheSetup(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Postgres, version: "17"})
	if out.Tuning.Mode != run.TuningAuto || out.Tuning.Durability != run.DurabilityFsync {
		t.Fatalf("tuning = %+v", out.Tuning)
	}
	for _, m := range out.Machines {
		if m.Role != "db" {
			continue
		}
		if m.CacheMB == 0 || m.DiskWriteIOPS != 7000 || m.DiskWriteMBps != 105 {
			t.Errorf("%s: cache %d MB, disk %d IOPS %d MB/s", m.Name, m.CacheMB, m.DiskWriteIOPS, m.DiskWriteMBps)
		}
	}
	relaxed, _ := compileTuned(t, tuneCase{
		kind: catalog.Postgres, version: "17",
		configs: map[string]map[string]json.RawMessage{"db": {"cfg.postgresql.conf@17": json.RawMessage(`{"synchronous_commit":"off"}`)}},
	})
	if relaxed.Tuning.Durability != run.DurabilityRelaxed {
		t.Errorf("synchronous_commit=off reads as %q", relaxed.Tuning.Durability)
	}
}

// The same input compiles to the same RunSpec: the entity diffs it, and a
// rendered map whose order changes would look like a change every time.
func TestAutoTuningIsDeterministic(t *testing.T) {
	for _, tc := range []tuneCase{
		{kind: catalog.Postgres, version: "17"},
		{kind: catalog.OrioleDB, version: "beta17-pg17"},
		{kind: catalog.MySQL, version: "8.4", protocol: catalog.ProtoMySQL},
		{kind: catalog.MariaDB, version: "11.4", protocol: catalog.ProtoMySQL},
		{kind: catalog.Cockroach, version: "25.4", protocol: catalog.ProtoCockroach},
		picoCase("26.2", map[string]any{"tiers": picoTiers(3, 1, nil)}),
		{kind: catalog.YDB, version: "26.2", protocol: catalog.ProtoYDBGrpc},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			var first []byte
			for range 8 {
				out, _ := compileTuned(t, tc)
				out.Spec.RunID = ""
				raw, err := json.Marshal(out.Spec)
				if err != nil {
					t.Fatal(err)
				}
				if first == nil {
					first = raw
				} else if string(raw) != string(first) {
					t.Fatal("two compilations of the same input differ")
				}
			}
		})
	}
}
