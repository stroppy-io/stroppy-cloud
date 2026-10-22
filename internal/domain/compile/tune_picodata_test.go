package compile_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const picoConf = "/etc/stroppy/picodata.yaml"

func picoCase(version string, params map[string]any) tuneCase {
	return tuneCase{kind: catalog.Picodata, version: version, params: params, protocol: catalog.ProtoPicodata}
}

func picoTiers(instances, rf int, extra map[string]any) []any {
	t := map[string]any{"name": "default", "instances": instances, "replication_factor": rf}
	for k, v := range extra {
		t[k] = v
	}
	return []any{t}
}

// picoInit is the init sidecar's script.
func picoInit(t *testing.T, out compile.Output) string {
	t.Helper()
	for _, c := range out.Spec.Containers {
		if strings.HasSuffix(c.Name, "-init") {
			return strings.Join(c.Cmd, " ")
		}
	}
	t.Fatal("no init sidecar")
	return ""
}

func picoContainers(out compile.Output) []spec.Container {
	var cs []spec.Container
	for _, c := range out.Spec.Containers {
		if c.Kind == spec.ContainerKindDatabase && strings.Contains(c.Name, "-picodata") {
			cs = append(cs, c)
		}
	}
	return cs
}

func TestPicodataAutoMemtx(t *testing.T) {
	for _, tc := range []struct {
		version, size string
		perMachine    int
		want          string
	}{
		// ((RAM − max(2G, 8%)) / n − 256M system − 256M) / 1.15, down to 256M
		{"26.1", "S", 1, "11776M"},
		{"26.1", "M", 1, "25600M"},
		{"26.2", "L", 1, "51968M"},
		{"26.2", "M", 4, "5888M"},
		{"26.1", "L", 4, "12544M"},
		// 25.3 has no separate system arena
		{"25.3", "M", 1, "25856M"},
	} {
		t.Run(fmt.Sprintf("%s/%s/%d", tc.version, tc.size, tc.perMachine), func(t *testing.T) {
			c := picoCase(tc.version, map[string]any{"tiers": picoTiers(tc.perMachine, 1, nil), "instances_per_machine": tc.perMachine})
			c.size = tc.size
			out, eff := compileTuned(t, c)
			cs := picoContainers(out)
			if len(cs) != tc.perMachine {
				t.Fatalf("%d instances", len(cs))
			}
			for _, ct := range cs {
				if conf := fileOf(out, ct.Name, picoConf); !strings.Contains(conf, "    memory: "+tc.want+"\n") {
					t.Fatalf("%s: memtx not %s:\n%s", ct.Name, tc.want, conf)
				}
			}
			var cfg map[string]any
			for id, raw := range eff["db"] {
				if strings.HasPrefix(id, "cfg.picodata.yaml@") {
					_ = json.Unmarshal(raw, &cfg)
				}
			}
			if cfg["memtx_memory"] != tc.want {
				t.Fatalf("effective memtx_memory = %v", cfg["memtx_memory"])
			}
		})
	}
}

func TestPicodataExplicitMemtxWins(t *testing.T) {
	out, _ := compileTuned(t, picoCase("26.1", map[string]any{"tiers": picoTiers(1, 1, nil), "memtx_memory_mb": 8192}))
	if conf := fileOf(out, "-picodata", picoConf); !strings.Contains(conf, "    memory: 8192M\n") {
		t.Fatalf("param memtx lost:\n%s", conf)
	}
	c := picoCase("26.1", map[string]any{"tiers": picoTiers(1, 1, nil)})
	c.configs = map[string]map[string]json.RawMessage{"db": {"cfg.picodata.yaml@26.1": json.RawMessage(`{"memtx_memory":"4G"}`)}}
	out, _ = compileTuned(t, c)
	if conf := fileOf(out, "-picodata", picoConf); !strings.Contains(conf, "    memory: 4G\n") {
		t.Fatalf("config memtx lost:\n%s", conf)
	}
	if _, ok := out.Tuned["db"]["cfg.picodata.yaml@26.1"]["memtx_memory"]; ok {
		t.Fatal("auto recorded over an explicit key")
	}
}

func TestPicodataWALMode(t *testing.T) {
	for _, tc := range []struct {
		version, tuning string
		tier            map[string]any
		want            string // "" = no wal_mode key
	}{
		{"26.2", "auto", nil, "fsync"},
		{"26.2", "stock", nil, "write"},
		{"26.2", "auto", map[string]any{"wal_mode": "write"}, "write"},
		{"26.2", "stock", map[string]any{"wal_mode": "fsync"}, "fsync"},
		{"26.1", "auto", nil, ""},
		{"25.3", "auto", nil, ""},
		{"26.1", "auto", map[string]any{"wal_mode": "write"}, ""},
	} {
		t.Run(tc.version+"/"+tc.tuning, func(t *testing.T) {
			out, _ := compileTuned(t, picoCase(tc.version, map[string]any{"tiers": picoTiers(1, 1, tc.tier), "tuning": tc.tuning}))
			conf := fileOf(out, "-picodata", picoConf)
			switch {
			case tc.want == "" && strings.Contains(conf, "wal_mode"):
				t.Fatalf("wal_mode rendered on %s:\n%s", tc.version, conf)
			case tc.want != "" && !strings.Contains(conf, "      wal_mode: "+tc.want+"\n"):
				t.Fatalf("wal_mode not %s:\n%s", tc.want, conf)
			}
		})
	}
}

func TestPicodataBucketCount(t *testing.T) {
	tiers := []any{
		map[string]any{"name": "default", "instances": 1},
		map[string]any{"name": "cold", "instances": 1, "bucket_count": 9000, "can_vote": false},
	}
	out, _ := compileTuned(t, picoCase("26.1", map[string]any{"tiers": tiers, "default_bucket_count": 6000}))
	conf := fileOf(out, "db-1-picodata", picoConf)
	for _, want := range []string{
		"  default_bucket_count: 6000\n",
		"    default:\n      replication_factor: 1\n      can_vote: true\n      bucket_count: 6000\n",
		"    cold:\n      replication_factor: 1\n      can_vote: false\n      bucket_count: 9000\n",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("missing %q:\n%s", want, conf)
		}
	}
}

func TestPicodataSQLSettings(t *testing.T) {
	auto := []string{
		"sql_vdbe_opcode_max = 0", "sql_storage_cache_count_max = 200",
		"sql_storage_cache_size_max = 33554432", "sql_motion_row_max = 50000",
	}
	for _, tc := range []struct {
		name    string
		version string
		params  map[string]any
		want    []string
		absent  []string
	}{
		{"single", "26.1", map[string]any{"tiers": picoTiers(1, 1, nil)}, append([]string{"sql_runtime_concurrency_max = 192"}, auto...), nil},
		{"three-behind-haproxy", "26.2", map[string]any{"tiers": picoTiers(3, 3, nil), "haproxy": 1}, append([]string{"sql_runtime_concurrency_max = 64"}, auto...), nil},
		{"25.3-has-no-concurrency", "25.3", map[string]any{"tiers": picoTiers(1, 1, nil)}, auto, []string{"sql_runtime_concurrency_max"}},
		{"stock", "26.1", map[string]any{"tiers": picoTiers(1, 1, nil), "tuning": "stock"}, []string{"sql_vdbe_opcode_max = 0"},
			[]string{"sql_runtime_concurrency_max", "sql_storage_cache", "sql_motion_row_max"}},
		{"explicit", "26.1", map[string]any{"tiers": picoTiers(1, 1, nil), "tuning": "stock", "sql_runtime_concurrency_max": 500, "sql_storage_cache_size_max_mb": 8},
			[]string{"sql_runtime_concurrency_max = 500", "sql_storage_cache_size_max = 8388608"}, []string{"sql_motion_row_max"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, eff := compileTuned(t, picoCase(tc.version, tc.params))
			init := picoInit(t, out)
			for _, w := range tc.want {
				if !strings.Contains(init, "assert(pico.sql([[ALTER SYSTEM SET "+w+"]]))") {
					t.Fatalf("missing %q in:\n%s", w, init)
				}
			}
			for _, a := range tc.absent {
				if strings.Contains(init, a) {
					t.Fatalf("unexpected %q in:\n%s", a, init)
				}
			}
			if tc.name == "single" {
				var params map[string]any
				_ = json.Unmarshal(eff["db"]["db.picodata.params@1"], &params)
				if params["sql_runtime_concurrency_max"] != float64(192) {
					t.Fatalf("effective params: %v", params)
				}
			}
		})
	}
}

func TestPicodataInstancesPerMachine(t *testing.T) {
	for _, tc := range []struct {
		name      string
		instances int
		rf        int
		haproxy   int
		machines  int
	}{
		{"one-machine", 4, 1, 0, 1},
		{"three-machines", 12, 3, 1, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{"tiers": picoTiers(tc.instances, tc.rf, nil), "instances_per_machine": 4, "haproxy": tc.haproxy}
			out, _ := compileTuned(t, picoCase("26.2", params))
			dbMachines := 0
			for _, m := range out.Spec.Machines {
				if m.Role == "db" {
					dbMachines++
				}
			}
			if dbMachines != tc.machines {
				t.Fatalf("%d db machines", dbMachines)
			}
			cs := picoContainers(out)
			if len(cs) != tc.instances {
				t.Fatalf("%d instances", len(cs))
			}
			ports := map[string]bool{}
			names := map[string]bool{}
			for _, c := range cs {
				if !strings.HasPrefix(c.Name, c.Machine+"-picodata-") {
					t.Fatalf("container name %s", c.Name)
				}
				for _, p := range c.Ports {
					key := fmt.Sprintf("%s:%d", c.Machine, p.Host)
					if ports[key] {
						t.Fatalf("port %s taken twice", key)
					}
					ports[key] = true
				}
				conf := fileOf(out, c.Name, picoConf)
				if !strings.Contains(conf, "  failure_domain: {host: "+c.Machine+"}\n") {
					t.Fatalf("%s: no failure domain:\n%s", c.Name, conf)
				}
				if strings.Count(conf, "${ip:db-") < tc.instances {
					t.Fatalf("%s: peers miss instances:\n%s", c.Name, conf)
				}
				if c.Mounts[0].Source == "/data/picodata" || names[c.Mounts[0].Source+c.Machine] {
					t.Fatalf("%s: shared instance directory %s", c.Name, c.Mounts[0].Source)
				}
				names[c.Mounts[0].Source+c.Machine] = true
			}
			for k := 0; k < 4; k++ {
				if !ports[fmt.Sprintf("db-1:%d", 4327+k)] || !ports[fmt.Sprintf("db-1:%d", 3301+k)] || !ports[fmt.Sprintf("db-1:%d", 8081+k)] {
					t.Fatalf("slot %d ports missing: %v", k, ports)
				}
			}
			if tc.haproxy == 0 {
				return
			}
			cfg := fileOf(out, "-haproxy", "/usr/local/etc/haproxy/haproxy.cfg")
			for m := 1; m <= tc.machines; m++ {
				for k := 0; k < 4; k++ {
					if !strings.Contains(cfg, fmt.Sprintf("${ip:db-%d}:%d check port %d ", m, 4327+k, 4327+k)) {
						t.Fatalf("haproxy misses db-%d:%d:\n%s", m, 4327+k, cfg)
					}
				}
			}
			for _, c := range out.Spec.Containers {
				if c.Kind == spec.ContainerKindProxy && len(c.DependsOn) != tc.instances {
					t.Fatalf("proxy waits for %v", c.DependsOn)
				}
			}
			if !strings.Contains(picoInit(t, out), "sql_runtime_concurrency_max = 50]]") {
				t.Fatal("12 endpoints at 128 VUs keep the floor of 50")
			}
		})
	}
}

func TestPicodataStockUnchanged(t *testing.T) {
	out, _ := compileTuned(t, picoCase("26.2", map[string]any{"tiers": picoTiers(1, 1, nil), "tuning": "stock"}))
	conf := fileOf(out, "db-1-picodata", picoConf)
	for _, want := range []string{"    memory: 2G\n", "      wal_mode: write\n", "      bucket_count: 3000\n", "  iproto:\n    listen: 0.0.0.0:3301\n"} {
		if !strings.Contains(conf, want) {
			t.Fatalf("missing %q:\n%s", want, conf)
		}
	}
	if strings.Contains(conf, "failure_domain") {
		t.Fatal("failure domain with one instance per machine")
	}
	if len(out.Tuned) != 0 {
		t.Fatalf("stock tuned: %v", out.Tuned)
	}
}
