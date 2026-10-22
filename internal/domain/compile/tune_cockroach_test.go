package compile_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const crdbFlags = "cfg.cockroach.flags@25"

// crdbNode is the first Cockroach node container.
func crdbNode(t *testing.T, out compile.Output) spec.Container {
	t.Helper()
	for _, c := range out.Spec.Containers {
		if strings.HasSuffix(c.Name, "-cockroach") {
			return c
		}
	}
	t.Fatal("no cockroach container")
	return spec.Container{}
}

// crdbFlag is the value of --name= on a node's command line.
func crdbFlag(c spec.Container, name string) string {
	for _, a := range c.Cmd {
		if v, ok := strings.CutPrefix(a, "--"+name+"="); ok {
			return v
		}
	}
	return ""
}

// crdbInit is the init sidecar's script.
func crdbInit(t *testing.T, out compile.Output) string {
	t.Helper()
	for _, c := range out.Spec.Containers {
		if strings.HasSuffix(c.Name, "-init") {
			return strings.Join(c.Cmd, " ")
		}
	}
	t.Fatal("no init sidecar")
	return ""
}

func TestCockroachAutoSizesFromMachine(t *testing.T) {
	for _, tc := range []struct {
		size, cache, temp string
		diskGB            int
	}{
		{"S", "30%", "15GiB", 100},
		{"M", "35%", "30GiB", 200},
		{"L", "35%", "32GiB", 0}, // the size's own io-m3 disk (93 GB blocks)
	} {
		t.Run(tc.size, func(t *testing.T) {
			out, eff := compileTuned(t, tuneCase{kind: catalog.Cockroach, version: "25.4", params: map[string]any{"nodes": 1}, size: tc.size, diskGB: tc.diskGB})
			node := crdbNode(t, out)
			disk := 0
			for _, m := range out.Spec.Machines {
				if m.Name == node.Machine {
					disk = m.Disks[0].GB
				}
			}
			if got := crdbFlag(node, "cache"); got != tc.cache {
				t.Errorf("--cache = %s, want %s", got, tc.cache)
			}
			if got := crdbFlag(node, "max-sql-memory"); got != "20%" {
				t.Errorf("--max-sql-memory = %s", got)
			}
			store := crdbFlag(node, "store")
			if !strings.Contains(store, "size=80%") {
				t.Errorf("--store = %s", store)
			}
			temp := crdbFlag(node, "max-disk-temp-storage")
			if temp != tc.temp {
				t.Errorf("--max-disk-temp-storage = %s, want %s", temp, tc.temp)
			}
			// The store and the temp budget fit the data disk together.
			tempGiB, _ := strconv.Atoi(strings.TrimSuffix(temp, "GiB"))
			if disk == 0 || 80*disk/100+tempGiB > disk {
				t.Errorf("store 80%% of %d GB + %d GiB temp overflow the disk", disk, tempGiB)
			}
			var cfg map[string]any
			_ = json.Unmarshal(eff["db"][crdbFlags], &cfg)
			if cfg["cache"] != tc.cache || cfg["max_disk_temp_storage"] != tc.temp {
				t.Errorf("effective configs: cache %v, temp %v", cfg["cache"], cfg["max_disk_temp_storage"])
			}
		})
	}
}

func TestCockroachAutoClusterSettings(t *testing.T) {
	out, eff := compileTuned(t, tuneCase{kind: catalog.Cockroach, version: "25.4", params: map[string]any{"nodes": 1}})
	sql := fileOf(out, "-init", "/etc/stroppy/settings.sql")
	for _, want := range []string{
		"SET CLUSTER SETTING kv.snapshot_rebalance.max_rate = '128MiB';",
		"SET CLUSTER SETTING kv.range_merge.queue.enabled = false;",
		"SET CLUSTER SETTING diagnostics.reporting.enabled = false;",
		"SET CLUSTER SETTING sql.txn.repeatable_read_isolation.enabled = true;",
		"ALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds = 600;",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("settings.sql lacks %q:\n%s", want, sql)
		}
	}
	for _, unsafe := range []string{"rocksdb.min_wal_sync_interval", "unsafe"} {
		if strings.Contains(sql, unsafe) {
			t.Errorf("settings.sql sets %s", unsafe)
		}
	}
	if !strings.Contains(crdbInit(t, out), "-f /etc/stroppy/settings.sql") {
		t.Error("the sidecar does not apply settings.sql")
	}
	var cfg map[string]any
	_ = json.Unmarshal(eff["db"][crdbFlags], &cfg)
	if s, _ := cfg["cluster_settings"].(map[string]any); s["diagnostics.reporting.enabled"] != "false" {
		t.Errorf("effective cluster settings: %v", cfg["cluster_settings"])
	}

	// 24.1 has no REPEATABLE READ setting of its own: left alone there.
	out, _ = compileTuned(t, tuneCase{kind: catalog.Cockroach, version: "24.1", params: map[string]any{"nodes": 1}})
	if sql := fileOf(out, "-init", "/etc/stroppy/settings.sql"); strings.Contains(sql, "isolation") || !strings.Contains(sql, "gc.ttlseconds = 600") {
		t.Errorf("24.1 settings.sql:\n%s", sql)
	}
}

func TestCockroachAutoThreeNodes(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Cockroach, version: "25.4", params: map[string]any{"nodes": 3, "haproxy": 1}})
	cfg := fileOf(out, "-haproxy", "/usr/local/etc/haproxy/haproxy.cfg")
	for _, want := range []string{"option httpchk", "http-check send meth GET uri /health?ready=1", "check port 8080"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("haproxy.cfg lacks %q:\n%s", want, cfg)
		}
	}
	// A joining node is not ready before init: its own check stays on
	// liveness, the sidecar waits for readiness of every node.
	if hc := strings.Join(crdbNode(t, out).Healthcheck.Cmd, " "); strings.Contains(hc, "ready=1") {
		t.Errorf("joining node checks readiness: %s", hc)
	}
	script := crdbInit(t, out)
	for i := 1; i <= 3; i++ {
		if want := fmt.Sprintf("until curl -sf 'http://${ip:db-%d}:8080/health?ready=1'", i); !strings.Contains(script, want) {
			t.Errorf("sidecar does not wait for db-%d readiness:\n%s", i, script)
		}
	}
	wait := strings.Index(script, "ranges.underreplicated")
	if wait < 0 || wait < strings.Index(script, "settings.sql") {
		t.Errorf("sidecar does not wait for replication after the settings:\n%s", script)
	}
	flows := 0
	for _, f := range out.Spec.Flows {
		if f.FromRole == "proxy" && f.Port == 8080 {
			flows++
		}
	}
	if flows != 1 {
		t.Errorf("proxy → db HTTP check flows: %d", flows)
	}
}

func TestCockroachAutoSingleNodeReadiness(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Cockroach, version: "25.4", params: map[string]any{"nodes": 1}})
	if hc := strings.Join(crdbNode(t, out).Healthcheck.Cmd, " "); !strings.Contains(hc, "'http://127.0.0.1:8080/health?ready=1'") {
		t.Errorf("single node healthcheck: %s", hc)
	}
	if strings.Contains(crdbInit(t, out), "underreplicated") {
		t.Error("a single node waits for replication")
	}
}

func TestCockroachExplicitConfigWins(t *testing.T) {
	own := json.RawMessage(`{"cache":"40%","max_disk_temp_storage":"8GiB","gc_ttl_seconds":3600,"cluster_settings":{"kv.snapshot_rebalance.max_rate":"'64MiB'"}}`)
	out, eff := compileTuned(t, tuneCase{
		kind: catalog.Cockroach, version: "25.4", params: map[string]any{"nodes": 1},
		configs: map[string]map[string]json.RawMessage{"db": {crdbFlags: own}},
	})
	node := crdbNode(t, out)
	if crdbFlag(node, "cache") != "40%" || crdbFlag(node, "max-disk-temp-storage") != "8GiB" {
		t.Errorf("explicit flags lost: %v", node.Cmd)
	}
	if crdbFlag(node, "max-sql-memory") != "20%" {
		t.Errorf("auto flag next to an explicit one: %v", node.Cmd)
	}
	sql := fileOf(out, "-init", "/etc/stroppy/settings.sql")
	if !strings.Contains(sql, "kv.snapshot_rebalance.max_rate = '64MiB';") || strings.Contains(sql, "128MiB") {
		t.Errorf("explicit cluster setting lost:\n%s", sql)
	}
	if !strings.Contains(sql, "gc.ttlseconds = 3600;") || !strings.Contains(sql, "diagnostics.reporting.enabled = false;") {
		t.Errorf("settings.sql:\n%s", sql)
	}
	var cfg map[string]any
	_ = json.Unmarshal(eff["db"][crdbFlags], &cfg)
	if cfg["cache"] != "40%" {
		t.Errorf("effective cache %v", cfg["cache"])
	}
}

func TestCockroachStockUnchanged(t *testing.T) {
	out, _ := compileTuned(t, tuneCase{kind: catalog.Cockroach, version: "25.4", params: map[string]any{"nodes": 3, "haproxy": 1, "tuning": "stock"}})
	node := crdbNode(t, out)
	if crdbFlag(node, "cache") != "25%" || crdbFlag(node, "max-sql-memory") != "25%" ||
		!strings.Contains(crdbFlag(node, "store"), "size=90%") || crdbFlag(node, "max-disk-temp-storage") != "32GiB" {
		t.Errorf("stock flags: %v", node.Cmd)
	}
	if hc := strings.Join(node.Healthcheck.Cmd, " "); strings.Contains(hc, "ready=1") {
		t.Errorf("stock healthcheck: %s", hc)
	}
	if sql := fileOf(out, "-init", "/etc/stroppy/settings.sql"); strings.Contains(sql, "SET CLUSTER SETTING") || strings.Contains(sql, "ALTER RANGE") {
		t.Errorf("stock settings.sql:\n%s", sql)
	}
	script := crdbInit(t, out)
	if strings.Contains(script, "settings.sql") || strings.Contains(script, "underreplicated") || strings.Contains(script, "curl") {
		t.Errorf("stock sidecar:\n%s", script)
	}
	if cfg := fileOf(out, "-haproxy", "/usr/local/etc/haproxy/haproxy.cfg"); !strings.Contains(cfg, "option tcp-check") {
		t.Errorf("stock haproxy check:\n%s", cfg)
	}
	if len(out.Tuned) != 0 {
		t.Errorf("stock recorded auto values: %v", out.Tuned)
	}
}
