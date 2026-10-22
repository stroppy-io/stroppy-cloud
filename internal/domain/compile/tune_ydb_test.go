package compile_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const ydbConf = "/opt/ydb/cfg/config.yaml"

// ydbNodeConf is the parsed config.yaml of one YDB node container.
type ydbNodeConf struct {
	Actor struct {
		Auto     bool   `yaml:"use_auto_config"`
		NodeType string `yaml:"node_type"`
		CPU      int    `yaml:"cpu_count"`
	} `yaml:"actor_system_config"`
	Memory struct {
		HardLimit int64 `yaml:"hard_limit_bytes"`
		Soft      int   `yaml:"soft_limit_percent"`
		Target    int   `yaml:"target_utilization_percent"`
		CacheMin  int   `yaml:"shared_cache_min_percent"`
		CacheMax  int   `yaml:"shared_cache_max_percent"`
	} `yaml:"memory_controller_config"`
	Broker *struct {
		Queues []struct {
			Name  string `yaml:"name"`
			Limit struct {
				CPU int `yaml:"cpu"`
			} `yaml:"limit"`
		} `yaml:"queues"`
	} `yaml:"resource_broker_config"`
	HostConfigs []struct {
		Drive []struct {
			Path string `yaml:"path"`
			Type string `yaml:"type"`
		} `yaml:"drive"`
	} `yaml:"host_configs"`
}

func ydbNode(t *testing.T, out compile.Output, machine string) ydbNodeConf {
	t.Helper()
	raw := fileOf(out, machine+"-ydb", ydbConf)
	if raw == "" {
		t.Fatalf("%s: no config.yaml", machine)
	}
	var conf ydbNodeConf
	if err := yaml.Unmarshal([]byte(raw), &conf); err != nil {
		t.Fatalf("%s: %v\n%s", machine, err, raw)
	}
	return conf
}

func ydbCase(params map[string]any) tuneCase {
	return tuneCase{kind: catalog.YDB, version: "26.2", protocol: catalog.ProtoYDBGrpc, params: params}
}

func TestYDBAutoCPUCountPerRoleAndSize(t *testing.T) {
	for _, tc := range []struct {
		size             string
		storage, compute int
	}{
		{"S", 3, 4},   // 4 vCPU
		{"M", 7, 7},   // 8 vCPU
		{"L", 14, 15}, // 16 vCPU
	} {
		t.Run(tc.size, func(t *testing.T) {
			c := ydbCase(nil)
			c.size = tc.size
			out, _ := compileTuned(t, c)
			s, d := ydbNode(t, out, "db-1"), ydbNode(t, out, "db-compute-1")
			if s.Actor.CPU != tc.storage || s.Actor.NodeType != "STORAGE" || !s.Actor.Auto {
				t.Errorf("storage actor system: %+v, want cpu %d", s.Actor, tc.storage)
			}
			if d.Actor.CPU != tc.compute || d.Actor.NodeType != "COMPUTE" || !d.Actor.Auto {
				t.Errorf("compute actor system: %+v, want cpu %d", d.Actor, tc.compute)
			}
		})
	}
}

func TestYDBAutoMemory(t *testing.T) {
	out, effective := compileTuned(t, ydbCase(nil)) // M: 32 GB
	want := int64(30) << 30
	s, d := ydbNode(t, out, "db-1"), ydbNode(t, out, "db-compute-1")
	if s.Memory.HardLimit != want || d.Memory.HardLimit != want {
		t.Errorf("hard_limit_bytes: storage %d, compute %d, want %d", s.Memory.HardLimit, d.Memory.HardLimit, want)
	}
	if d.Memory.CacheMin != 25 || d.Memory.CacheMax != 60 || d.Memory.Target != 60 || d.Memory.Soft != 75 {
		t.Errorf("compute shared cache: %+v", d.Memory)
	}
	if d.Memory.CacheMin+1+30 >= d.Memory.Soft || d.Memory.CacheMin+1 >= d.Memory.Target {
		t.Errorf("memory controller invariants broken: %+v", d.Memory)
	}
	if s.Memory.CacheMin != 20 || s.Memory.CacheMax != 50 || s.Memory.Target != 50 {
		t.Errorf("storage keeps the upstream cache shares: %+v", s.Memory)
	}
	var tuned map[string]any
	_ = json.Unmarshal(effective["db-compute"]["cfg.ydb.config.yaml@26"], &tuned)
	if numberOf(tuned["memory_hard_limit_bytes"]) != float64(want) || numberOf(tuned["cpu_count"]) != 7 {
		t.Errorf("effective configs miss the auto values: %v", tuned)
	}

	c := ydbCase(nil)
	c.size = "L" // 64 GB
	out, _ = compileTuned(t, c)
	if got := ydbNode(t, out, "db-compute-1").Memory.HardLimit; got != int64(61)<<30 {
		t.Errorf("64 GB hard_limit_bytes = %d", got)
	}
}

func TestYDBAutoResourceBroker(t *testing.T) {
	out, _ := compileTuned(t, ydbCase(nil))
	d := ydbNode(t, out, "db-compute-1")
	if d.Broker == nil || len(d.Broker.Queues) != 1 || d.Broker.Queues[0].Name != "queue_build_index" || d.Broker.Queues[0].Limit.CPU != 2 {
		t.Errorf("compute resource broker: %+v", d.Broker)
	}
	if ydbNode(t, out, "db-1").Broker != nil {
		t.Error("storage nodes build no indexes")
	}
}

// pdiskSizes is the truncate size of every pdisk file per storage machine.
func pdiskSizes(out compile.Output) map[string][]string {
	sizes := map[string][]string{}
	for _, h := range out.Spec.HostPrep {
		if h.Kind != spec.HostPrepScript || !strings.Contains(h.Content, "truncate") {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(h.Content), "\n") {
			if f := strings.Fields(line); len(f) > 7 && f[5] == "truncate" {
				sizes[h.Machine] = append(sizes[h.Machine], f[7])
			}
		}
	}
	return sizes
}

func dataDisk(out compile.Output, machine string) int {
	for _, m := range out.Spec.Machines {
		if m.Name == machine {
			for _, d := range m.Disks {
				if d.Mount == "/data" {
					return d.GB
				}
			}
		}
	}
	return 0
}

func TestYDBPdiskSizeFromDisk(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params map[string]any
		diskGB int
		want   string
		count  int
	}{
		{"single 300 GB", nil, 300, "232G", 1},           // 0.8 × 290
		{"single 100 GB", nil, 100, "80G", 1},            // 72 → floor
		{"three pdisks 600 GB", map[string]any{"pdisks_per_node": 3}, 600, "157G", 3},
		{"three pdisks at the minimum", map[string]any{"pdisks_per_node": 3}, 260, "80G", 3},
		{"fixed size", map[string]any{"auto_size_pdisks": false}, 300, "80G", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := ydbCase(tc.params)
			c.diskGB = tc.diskGB
			out, _ := compileTuned(t, c)
			got := pdiskSizes(out)["db-1"]
			if len(got) != tc.count {
				t.Fatalf("pdisk files: %v", got)
			}
			for _, g := range got {
				if g != tc.want {
					t.Errorf("pdisk size %s, want %s", g, tc.want)
				}
			}
		})
	}
	// Without a chosen disk the machine is grown to hold every pdisk.
	out, _ := compileTuned(t, ydbCase(map[string]any{"pdisks_per_node": 4}))
	if disk := dataDisk(out, "db-1"); disk < 4*80+20 {
		t.Errorf("storage disk %d GB under four 80 GB pdisks", disk)
	}
	for _, g := range pdiskSizes(out)["db-1"] {
		if g != fmt.Sprintf("%dG", max(80, int(0.8*float64(dataDisk(out, "db-1")-10)/4))) {
			t.Errorf("pdisk %s on a %d GB disk", g, dataDisk(out, "db-1"))
		}
	}
}

// createGroups is the storage pool and group count the init sidecar asks for.
func createGroups(out compile.Output) string {
	for _, c := range out.Spec.Containers {
		if strings.HasSuffix(c.Name, "-init") {
			cmd := strings.Join(c.Cmd, " ")
			if i := strings.Index(cmd, " create "); i >= 0 {
				return strings.Fields(cmd[i+len(" create "):])[0]
			}
		}
	}
	return ""
}

func TestYDBStorageGroups(t *testing.T) {
	mirror := map[string]any{"fault_tolerance": "mirror-3-dc", "storage_nodes": 9, "database_nodes": 3, "pdisks_per_node": 3}
	withGroups := map[string]any{"storage_groups": 3}
	for k, v := range mirror {
		withGroups[k] = v
	}
	for _, tc := range []struct {
		name   string
		params map[string]any
		size   string
		want   string
	}{
		{"single auto", nil, "M", "ssd:4;"},
		{"single stock", map[string]any{"tuning": "stock"}, "M", "ssd:1;"},
		{"single set", map[string]any{"storage_groups": 8}, "M", "ssd:8;"},
		{"mirror-3-dc 3 × 8 vCPU", mirror, "M", "ssd:12;"},
		{"mirror-3-dc 3 × 4 vCPU", mirror, "S", "ssd:6;"},
		{"mirror-3-dc 3 × 16 vCPU", mirror, "L", "ssd:23;"}, // 24 wanted, 27 pdisks × 8 slots hold 23
		{"mirror-3-dc set", withGroups, "M", "ssd:3;"},
		{"nvme", map[string]any{"disk_type": "NVME"}, "M", "nvme:4;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := ydbCase(tc.params)
			c.size = tc.size
			out, _ := compileTuned(t, c)
			if got := createGroups(out); got != tc.want {
				t.Errorf("database create %s, want %s", got, tc.want)
			}
		})
	}
}

func TestYDBDiskTypeReachesConfig(t *testing.T) {
	out, _ := compileTuned(t, ydbCase(map[string]any{"disk_type": "NVME"}))
	raw := fileOf(out, "db-1-ydb", ydbConf)
	for _, want := range []string{"type: NVME", "kind: nvme", "storage_pool_kind: nvme", `"pdisk_category":"NVME"`} {
		if !strings.Contains(raw, want) {
			t.Errorf("config.yaml lacks %q", want)
		}
	}
}

func TestYDBExplicitConfigWins(t *testing.T) {
	c := ydbCase(nil)
	c.configs = map[string]map[string]json.RawMessage{
		"db-compute": {"cfg.ydb.config.yaml@26": json.RawMessage(`{"cpu_count":3,"memory_shared_cache_max_percent":40,"memory_soft_limit_percent":50}`)},
	}
	out, effective := compileTuned(t, c)
	d := ydbNode(t, out, "db-compute-1")
	if d.Actor.CPU != 3 || d.Memory.CacheMax != 40 {
		t.Errorf("explicit keys lost: %+v %+v", d.Actor, d.Memory)
	}
	// soft 50: a 25% cache floor would break min + mem_table + activities < soft,
	// and a 60% target would pass the soft limit.
	if d.Memory.CacheMin != 20 || d.Memory.Target != 50 {
		t.Errorf("auto shares break the explicit soft limit: %+v", d.Memory)
	}
	var tuned map[string]any
	_ = json.Unmarshal(effective["db-compute"]["cfg.ydb.config.yaml@26"], &tuned)
	if numberOf(tuned["cpu_count"]) != 3 {
		t.Errorf("effective cpu_count = %v", tuned["cpu_count"])
	}
}

func TestYDBStockUnchanged(t *testing.T) {
	out, _ := compileTuned(t, ydbCase(map[string]any{"tuning": "stock"}))
	for _, m := range []string{"db-1", "db-compute-1"} {
		n := ydbNode(t, out, m)
		if n.Actor.CPU != 4 || n.Memory.HardLimit != 0 || n.Memory.CacheMin != 20 || n.Memory.CacheMax != 50 || n.Memory.Target != 50 || n.Broker != nil {
			t.Errorf("%s: stock config tuned: %+v %+v", m, n.Actor, n.Memory)
		}
	}
	if len(out.Tuned) != 0 {
		t.Errorf("stock recorded auto values: %v", out.Tuned)
	}
	for _, c := range out.Spec.Containers {
		if c.Kind == spec.ContainerKindDatabase && len(c.Ulimits) > 0 {
			t.Errorf("%s: ulimits under stock tuning", c.Name)
		}
	}
	for _, h := range out.Spec.HostPrep {
		if h.Kind == spec.HostPrepSysctl {
			t.Fatal("stock tuning wrote a sysctl drop-in")
		}
	}
}

func TestYDBAutoHostTuning(t *testing.T) {
	out, _ := compileTuned(t, ydbCase(nil))
	sysctl, thp := map[string]bool{}, map[string]bool{}
	for _, h := range out.Spec.HostPrep {
		switch {
		case h.Kind == spec.HostPrepSysctl && strings.Contains(h.Content, "vm.overcommit_memory = 1"):
			sysctl[h.Machine] = true
		case h.Kind == spec.HostPrepScript && strings.Contains(h.Content, "echo madvise > \"$thp/enabled\""):
			thp[h.Machine] = true
		case h.Kind == spec.HostPrepScript && strings.Contains(h.Content, "echo never > \"$thp/enabled\""):
			t.Errorf("%s: THP never on a YDB node", h.Machine)
		}
	}
	for _, m := range []string{"db-1", "db-compute-1"} {
		if !sysctl[m] || !thp[m] {
			t.Errorf("%s: overcommit %v, THP madvise %v", m, sysctl[m], thp[m])
		}
	}
	for _, c := range out.Spec.Containers {
		if c.Kind != spec.ContainerKindDatabase {
			continue
		}
		if c.Ulimits["memlock"] != -1 || c.Ulimits["nofile"] < 65536 {
			t.Errorf("%s: ulimits %v", c.Name, c.Ulimits)
		}
	}
}

func numberOf(v any) float64 {
	n, _ := v.(float64)
	return n
}
