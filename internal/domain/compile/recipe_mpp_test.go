package compile_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type mppRelease struct {
	kind    catalog.DatabaseKind
	version string
	image   string
	keyword string
}

// mppKinds are the Greenplum family with the image of each version.
func mppKinds() []mppRelease {
	return []mppRelease{
		{catalog.Greenplum, "7.1.0", "docker.stroppy.io/stroppy-io/greenplum:7.1.0", "COORDINATOR"},
		{catalog.Greenplum, "6.27.1", "docker.stroppy.io/stroppy-io/greenplum:6.27.1", "MASTER"},
		{catalog.Greengage, "7.5.1", "docker.stroppy.io/stroppy-io/greengage:7.5.1", "COORDINATOR"},
		{catalog.Greengage, "6.32.0", "docker.stroppy.io/stroppy-io/greengage:6.32.0", "MASTER"},
		{catalog.Greengage, "8.0.0-alpha.1", "docker.stroppy.io/stroppy-io/greengage:8.0.0-alpha.1", "COORDINATOR"},
		{catalog.WarehousePG, "7.6.1", "docker.stroppy.io/stroppy-io/warehousepg:7.6.1-whpg", "COORDINATOR"},
		{catalog.WarehousePG, "6.27.7", "docker.stroppy.io/stroppy-io/warehousepg:6.27.7-whpg", "MASTER"},
		{catalog.OpenGPDB, "6.30.2", "docker.stroppy.io/stroppy-io/opengpdb:6.30.2", "MASTER"},
		{catalog.Cloudberry, "2.1.0", "docker.stroppy.io/stroppy-io/cloudberry:2.1.0-incubating", "COORDINATOR"},
	}
}

// mppCase is a cluster of the family: the coordinator takes size M and the
// segment hosts XL, the sizes the product starts from.
func mppCase(kind catalog.DatabaseKind, version string, params map[string]any) tuneCase {
	p := map[string]any{"segment_hosts": 4}
	for k, v := range params {
		p[k] = v
	}
	return tuneCase{kind: kind, version: version, params: p, vus: 1, roleSizes: map[string]string{"coordinator": "M", "db": "XL"}}
}

func containerNamed(t *testing.T, out compile.Output, name string) spec.Container {
	t.Helper()
	for _, c := range out.Spec.Containers {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no container %s", name)
	return spec.Container{}
}

func fileIn(c spec.Container, path string) string {
	for _, f := range c.Files {
		if f.Path == path {
			return f.Content
		}
	}
	return ""
}

func TestMPPClusterShape(t *testing.T) {
	for _, tc := range mppKinds() {
		t.Run(fmt.Sprintf("%s-%s", tc.kind, tc.version), func(t *testing.T) {
			t.Parallel()
			out, _ := compileTuned(t, mppCase(tc.kind, tc.version, nil))
			engine := string(tc.kind)
			if len(out.Spec.Machines) != 1+4+1 {
				t.Fatalf("%d machines, want coordinator + 4 segment hosts + runner", len(out.Spec.Machines))
			}
			coordinator := containerNamed(t, out, "coordinator-1-"+engine)
			if coordinator.Image != tc.image || coordinator.Kind != spec.ContainerKindDatabase || coordinator.Role != "coordinator" {
				t.Errorf("coordinator %+v", coordinator)
			}
			if coordinator.Env["MPP_SSH_PORT"] != "2222" || len(coordinator.Mounts) != 1 || coordinator.Mounts[0].Source != "/data/mpp" || coordinator.Mounts[0].Target != "/data/mpp" {
				t.Errorf("coordinator env %v mounts %v", coordinator.Env, coordinator.Mounts)
			}
			if len(coordinator.Ports) != 1 || coordinator.Ports[0].Host != 5432 {
				t.Errorf("coordinator ports %v", coordinator.Ports)
			}
			if strings.Join(coordinator.Cmd, " ") != "bash /etc/stroppy/mpp-init.sh" {
				t.Errorf("coordinator cmd %v", coordinator.Cmd)
			}
			if coordinator.Healthcheck == nil || len(coordinator.Healthcheck.Cmd) != 2 || !strings.Contains(coordinator.Healthcheck.Cmd[1], "mpp-healthcheck") || !strings.Contains(coordinator.Healthcheck.Cmd[1], "/tmp/stroppy-mpp-ready") {
				t.Errorf("the coordinator is healthy before the cluster is: %+v", coordinator.Healthcheck)
			}
			if len(coordinator.DependsOn) != 4 {
				t.Errorf("coordinator waits for %v", coordinator.DependsOn)
			}
			for i := 1; i <= 4; i++ {
				seg := containerNamed(t, out, fmt.Sprintf("db-%d-%s", i, engine))
				if seg.Image != tc.image || seg.Kind != spec.ContainerKindDatabase || len(seg.Cmd) != 0 || len(seg.Entrypoint) != 0 {
					t.Errorf("segment host %d: %+v", i, seg)
				}
				if seg.Healthcheck == nil || strings.Join(seg.Healthcheck.Cmd, " ") != "CMD mpp-healthcheck" {
					t.Errorf("segment host %d healthcheck %+v", i, seg.Healthcheck)
				}
				// XL is 32 vCPU: 16 segments, ports from 6000.
				if len(seg.Ports) != 16 || seg.Ports[0].Host != 6000 || seg.Ports[15].Host != 6015 {
					t.Errorf("segment host %d ports %v", i, seg.Ports)
				}
			}
			var exporters []string
			for _, c := range out.Spec.Containers {
				if strings.HasSuffix(c.Name, "-postgres-exporter") {
					exporters = append(exporters, c.Machine)
					if !strings.Contains(c.Env["DATA_SOURCE_NAME"], "stroppy:stroppy@127.0.0.1:5432/stroppy") {
						t.Errorf("exporter source %q", c.Env["DATA_SOURCE_NAME"])
					}
					cmd := " " + strings.Join(c.Cmd, " ") + " "
					for _, want := range []string{" --no-collector.wal ", " --no-collector.stat_user_tables ", " --collector.postmaster "} {
						if !strings.Contains(cmd, want) {
							t.Errorf("exporter flags lack %q: %s", want, cmd)
						}
					}
					for _, bad := range []string{"stat_statements", "process_idle", "stat_checkpointer", "long_running_transactions"} {
						if strings.Contains(cmd, "--collector."+bad+" ") {
							t.Errorf("exporter asks for %s, which the family cannot serve", bad)
						}
					}
					// A PostgreSQL 9.4 base has not the views of five more collectors.
					legacy := tc.keyword == "MASTER"
					for _, flag := range []string{" --no-collector.replication ", " --no-collector.replication_slot ", " --no-collector.stat_progress_vacuum "} {
						if strings.Contains(cmd, flag) != legacy {
							t.Errorf("%s%s: legacy base %v", tc.kind, tc.version, legacy)
						}
					}
					for _, collector := range []string{"database_wraparound", "stat_wal_receiver"} {
						if strings.Contains(cmd, " --collector."+collector+" ") == legacy {
							t.Errorf("%s %s: collector %s with legacy base %v", tc.kind, tc.version, collector, legacy)
						}
					}
				}
			}
			if len(exporters) != 1 || exporters[0] != "coordinator-1" {
				t.Errorf("postgres exporters on %v, want the coordinator only", exporters)
			}
			if out.ClientURL != "postgresql://stroppy:stroppy@${ip:role:coordinator}:5432/stroppy?sslmode=disable" {
				t.Errorf("client URL %q", out.ClientURL)
			}
			for _, m := range out.Spec.Machines {
				node := false
				for _, c := range out.Spec.Containers {
					node = node || (c.Machine == m.Name && strings.HasSuffix(c.Name, "-node-exporter"))
				}
				if !node {
					t.Errorf("%s has no node exporter", m.Name)
				}
			}
		})
	}
}

func TestMPPSSHKeyIsOneForTheCluster(t *testing.T) {
	out, _ := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", nil))
	coordinator := containerNamed(t, out, "coordinator-1-greengage")
	private := fileIn(coordinator, "/etc/mpp/ssh/id_ed25519")
	public := fileIn(coordinator, "/etc/mpp/ssh/id_ed25519.pub")
	if !strings.HasPrefix(private, "-----BEGIN OPENSSH PRIVATE KEY-----") || !strings.HasPrefix(public, "ssh-ed25519 ") {
		t.Fatalf("key files: %.40q %.40q", private, public)
	}
	for _, c := range out.Spec.Containers {
		if c.Kind != spec.ContainerKindDatabase {
			continue
		}
		// The key is accepted only from the cluster's machines.
		authorized := fileIn(c, "/etc/mpp/ssh/authorized_keys")
		if !strings.HasPrefix(authorized, `from="${ip:coordinator-1},${ip:db-1},`) || !strings.HasSuffix(authorized, `" `+public) {
			t.Errorf("%s: authorized_keys %q", c.Name, authorized)
		}
		if fileIn(c, "/etc/mpp/ssh/id_ed25519") != private || fileIn(c, "/etc/mpp/ssh/id_ed25519.pub") != public {
			t.Errorf("%s does not carry the cluster's key", c.Name)
		}
		for _, f := range c.Files {
			if f.Path == "/etc/mpp/ssh/id_ed25519" && f.Mode != "0600" {
				t.Errorf("%s: private key mode %q", c.Name, f.Mode)
			}
		}
	}
	// Another run, another key.
	other, _ := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", nil))
	if fileIn(containerNamed(t, other, "coordinator-1-greengage"), "/etc/mpp/ssh/id_ed25519") == private {
		t.Error("two runs share a key")
	}
}

func TestMPPInitScriptsFollowTheVersion(t *testing.T) {
	for _, tc := range mppKinds() {
		t.Run(fmt.Sprintf("%s-%s", tc.kind, tc.version), func(t *testing.T) {
			t.Parallel()
			out, _ := compileTuned(t, mppCase(tc.kind, tc.version, nil))
			coordinator := containerNamed(t, out, "coordinator-1-"+string(tc.kind))
			cfg := fileIn(coordinator, "/etc/stroppy/gpinitsystem_config")
			other := "MASTER"
			if tc.keyword == "MASTER" {
				other = "COORDINATOR"
			}
			for _, want := range []string{
				tc.keyword + "_HOSTNAME=${ip:coordinator-1}", tc.keyword + "_DIRECTORY=/data/mpp/coordinator", tc.keyword + "_PORT=5432",
				"PORT_BASE=6000", "ENCODING=UNICODE", "DATABASE_NAME=stroppy", "TRUSTED_SHELL=ssh",
				"declare -a DATA_DIRECTORY=(" + strings.TrimSuffix(strings.Repeat("/data/mpp/primary ", 16), " ") + ")",
			} {
				if !strings.Contains(cfg, want) {
					t.Errorf("gpinitsystem_config lacks %q:\n%s", want, cfg)
				}
			}
			if strings.Contains(cfg, other+"_") {
				t.Errorf("gpinitsystem_config speaks %s on %s %s:\n%s", other, tc.kind, tc.version, cfg)
			}
			script := fileIn(coordinator, "/etc/stroppy/mpp-init.sh")
			cluster := fileIn(coordinator, "/etc/stroppy/mpp-cluster.sh")
			onlyFlag, otherFlag := "--coordinatoronly", "--masteronly"
			if tc.keyword == "MASTER" {
				onlyFlag, otherFlag = otherFlag, onlyFlag
			}
			for _, want := range []string{
				"SELF=${ip:coordinator-1}", `HOSTS="${ip:coordinator-1} ${ip:db-1} ${ip:db-2} ${ip:db-3} ${ip:db-4}"`,
				"runuser -u gpadmin -- \"$@\"", "as_gpadmin ssh -o BatchMode=yes", "hostname", ">/etc/hosts", "mpp-cluster.sh", "touch \"$READY\"", "READY=/tmp/stroppy-mpp-ready",
			} {
				if !strings.Contains(script, want) {
					t.Errorf("mpp-init.sh lacks %q", want)
				}
			}
			for _, want := range []string{
				". /usr/local/mpp/env.sh", "cp /etc/stroppy/gpinitsystem_config \"$HOME/gpinitsystem_config\"", "gpinitsystem -a -c \"$HOME/gpinitsystem_config\" -h", "printf '%s\\n' ${ip:db-1} ${ip:db-2} ${ip:db-3} ${ip:db-4}",
				"gpconfig -c", onlyFlag, "gpstop -ar", "CREATE ROLE stroppy LOGIN PASSWORD 'stroppy';", "ALTER DATABASE stroppy OWNER TO stroppy;",
				"host all stroppy ${ip:role:runner}/32 md5", "gpstop -u", "export COORDINATOR_DATA_DIRECTORY=/data/mpp/coordinator/gpseg-1 MASTER_DATA_DIRECTORY=/data/mpp/coordinator/gpseg-1",
			} {
				if !strings.Contains(cluster, want) {
					t.Errorf("mpp-cluster.sh lacks %q", want)
				}
			}
			if strings.Contains(cluster, otherFlag) {
				t.Errorf("mpp-cluster.sh uses %s on a %s base", otherFlag, tc.keyword)
			}
		})
	}
}

func TestMPPSegmentsPerHost(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params map[string]any
		size   string
		want   int
	}{
		{"auto on XL (32 vCPU)", nil, "XL", 16},
		{"auto on S (4 vCPU)", nil, "S", 2},
		{"explicit", map[string]any{"segments_per_host": 6}, "XL", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mppCase(catalog.Greengage, "7.5.1", tc.params)
			c.roleSizes["db"] = tc.size
			out, _ := compileTuned(t, c)
			cfg := fileIn(containerNamed(t, out, "coordinator-1-greengage"), "/etc/stroppy/gpinitsystem_config")
			want := "DATA_DIRECTORY=(" + strings.TrimSuffix(strings.Repeat("/data/mpp/primary ", tc.want), " ") + ")"
			if !strings.Contains(cfg, want) {
				t.Errorf("want %d segments per host:\n%s", tc.want, cfg)
			}
			if got := len(containerNamed(t, out, "db-1-greengage").Ports); got != tc.want {
				t.Errorf("%d ports declared, want %d", got, tc.want)
			}
		})
	}
}

func gucsOf(t *testing.T, out compile.Output, role string) map[string]string {
	t.Helper()
	name := "mpp-gucs-segments.conf"
	machine := "coordinator-1-greengage"
	if role == "coordinator" {
		name = "mpp-gucs-coordinator.conf"
	}
	out2 := map[string]string{}
	for _, line := range strings.Split(fileIn(containerNamed(t, out, machine), "/etc/stroppy/"+name), "\n") {
		if k, v, ok := strings.Cut(line, " = "); ok && !strings.HasPrefix(line, "#") {
			out2[k] = v
		}
	}
	return out2
}

func TestMPPOptimizerParameter(t *testing.T) {
	for _, tc := range []struct {
		name, tuning, optimizer, want string
	}{
		{"orca", "auto", "orca", "on"},
		{"planner", "auto", "postgres", "off"},
		{"orca in stock", "stock", "orca", "on"},
		{"planner in stock", "stock", "postgres", "off"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", map[string]any{"tuning": tc.tuning, "optimizer": tc.optimizer}))
			for _, role := range []string{"coordinator", "db"} {
				if got := gucsOf(t, out, role)["optimizer"]; got != tc.want {
					t.Errorf("%s: optimizer = %q, want %q", role, got, tc.want)
				}
			}
		})
	}
}

func TestMPPStockTuningChangesNothingElse(t *testing.T) {
	out, _ := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", map[string]any{"tuning": "stock"}))
	for _, role := range []string{"coordinator", "db"} {
		if g := gucsOf(t, out, role); len(g) != 1 || g["optimizer"] != "on" {
			t.Errorf("%s: stock GUCs %v, want the optimizer only", role, g)
		}
	}
	for _, h := range out.Spec.HostPrep {
		if h.Kind == spec.HostPrepSysctl {
			t.Fatal("stock tuning wrote a sysctl drop-in")
		}
	}
	for _, c := range out.Spec.Containers {
		if len(c.Ulimits) > 0 {
			t.Errorf("%s: ulimits under stock tuning", c.Name)
		}
	}
}

// The auto GUCs of a 1 + 4 cluster with segment hosts of 32 vCPU / 128 GB
// (16 segments each) and a coordinator of 2 vCPU / 8 GB, one query at a time.
func TestMPPAutoGUCs(t *testing.T) {
	out, eff := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", nil))
	segments, coordinator := gucsOf(t, out, "db"), gucsOf(t, out, "coordinator")
	for k, want := range map[string]string{
		// 0.9 × 131072 MB / 16 segments.
		"gp_vmem_protect_limit": "7372", "max_statement_mem": "6634MB", "statement_mem": "5529MB",
		"work_mem": "819MB", "effective_cache_size": "4096MB", "runaway_detector_activation_percent": "100",
		"gp_resource_manager": "queue", "gp_autostats_mode": "none", "autovacuum": "off", "gp_max_packet_size": "1420",
		"gp_interconnect_queue_depth": "16", "gp_interconnect_snd_queue_depth": "16", "max_connections": "750",
		"gp_fts_probe_timeout": "600", "gp_fts_probe_interval": "600", "gp_segment_connect_timeout": "1800",
		"max_wal_size": "4096MB", "optimizer": "on",
	} {
		if segments[k] != want {
			t.Errorf("segments %s = %q, want %q", k, segments[k], want)
		}
	}
	if cfg := fileIn(containerNamed(t, out, "coordinator-1-greengage"), "/etc/stroppy/gpinitsystem_config"); strings.Contains(cfg, "CHECK_POINT_SEGMENTS") {
		t.Errorf("CHECK_POINT_SEGMENTS on a PostgreSQL 12 base:\n%s", cfg)
	}
	// The coordinator is one instance on 8 GB.
	for k, want := range map[string]string{"gp_vmem_protect_limit": "7372", "max_connections": "250", "statement_mem": "5529MB", "work_mem": "819MB"} {
		if coordinator[k] != want {
			t.Errorf("coordinator %s = %q, want %q", k, coordinator[k], want)
		}
	}
	if coordinator["effective_cache_size"] != "4096MB" {
		t.Errorf("coordinator effective_cache_size = %q", coordinator["effective_cache_size"])
	}
	var tuned map[string]any
	if raw := eff["db"]["cfg.mpp.gucs@1"]; json.Unmarshal(raw, &tuned) != nil || tuned["gp_vmem_protect_limit"] == nil {
		t.Errorf("the effective config of the segments lacks the auto values: %s", eff["db"]["cfg.mpp.gucs@1"])
	}
	// The 9.4 base spells the checkpoint knob differently.
	legacy, _ := compileTuned(t, mppCase(catalog.Greengage, "6.32.0", nil))
	g := gucsOf(t, legacy, "db")
	if _, ok := g["checkpoint_segments"]; ok || g["max_wal_size"] != "" {
		t.Errorf("6.x WAL GUCs %v", g)
	}
	if cfg := fileIn(containerNamed(t, legacy, "coordinator-1-greengage"), "/etc/stroppy/gpinitsystem_config"); strings.Contains(cfg, "CHECK_POINT_SEGMENTS") {
		t.Errorf("6.x gpinitsystem_config sets the checkpoint spacing on the coordinator only:\n%s", cfg)
	}
	// gpconfig refuses autovacuum on the 6.x engines.
	if _, ok := g["autovacuum"]; ok {
		t.Error("autovacuum is set on a 6.x engine")
	}
}

func TestMPPAutoKernel(t *testing.T) {
	out, _ := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", nil))
	byMachine := map[string]string{}
	for _, h := range out.Spec.HostPrep {
		if h.Kind == spec.HostPrepSysctl {
			byMachine[h.Machine] = h.Content
		}
	}
	if len(byMachine) != 5 {
		t.Fatalf("sysctl drop-ins on %v, want the coordinator and four segment hosts", byMachine)
	}
	for machine, want := range map[string][]string{
		// 128 GB: ratio 100 × (1 − 0.026 × 0.9), dirty ratios near 1.5 and 4 GiB, 3% free.
		"db-1": {"vm.overcommit_memory = 2", "vm.overcommit_ratio = 97", "vm.dirty_background_ratio = 1", "vm.dirty_ratio = 3", "vm.min_free_kbytes = 4026531"},
		// 8 GB: the vendor ratios of a host under 64 GB.
		"coordinator-1": {"vm.overcommit_memory = 2", "vm.overcommit_ratio = 97", "vm.dirty_background_ratio = 3", "vm.dirty_ratio = 10", "vm.min_free_kbytes = 251658"},
	} {
		for _, w := range want {
			if !strings.Contains(byMachine[machine], w) {
				t.Errorf("%s: sysctl lacks %q:\n%s", machine, w, byMachine[machine])
			}
		}
	}
	for _, c := range out.Spec.Containers {
		if c.Kind == spec.ContainerKindDatabase && c.Ulimits["nofile"] != 1048576 {
			t.Errorf("%s: nofile = %d", c.Name, c.Ulimits["nofile"])
		}
	}
}

// Stat of the analytic scripts only: the concurrency of the workload shares
// statement_mem.
func TestMPPStatementMemShrinksWithConcurrency(t *testing.T) {
	c := mppCase(catalog.Greengage, "7.5.1", nil)
	c.vus = 4
	out, _ := compileTuned(t, c)
	g := gucsOf(t, out, "db")
	// 0.75 × 7372 / 4.
	if g["statement_mem"] != "1382MB" || g["max_statement_mem"] != "6634MB" {
		t.Errorf("statement_mem %s, max_statement_mem %s", g["statement_mem"], g["max_statement_mem"])
	}
}

func TestMPPSummary(t *testing.T) {
	out, _ := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", nil))
	if out.Tuning.Mode != run.TuningAuto || out.Tuning.Durability != run.DurabilityFsync {
		t.Fatalf("tuning = %+v", out.Tuning)
	}
	if out.Tuning.Auto["db"]["cfg.mpp.gucs@1"]["gp_vmem_protect_limit"] != 7372 || out.Tuning.Auto["coordinator"]["cfg.mpp.gucs@1"]["optimizer"] != "on" {
		t.Errorf("auto values %v", out.Tuning.Auto)
	}
	stock, _ := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", map[string]any{"tuning": "stock"}))
	if stock.Tuning.Mode != run.TuningStock || len(stock.Tuning.Auto) != 0 {
		t.Errorf("stock tuning = %+v", stock.Tuning)
	}
}

func TestMPPSameInputSameSpec(t *testing.T) {
	// compileTuned draws a new run id; strip what depends on it.
	var first string
	for range 4 {
		out, _ := compileTuned(t, mppCase(catalog.Greengage, "7.5.1", nil))
		for i := range out.Spec.Containers {
			c := &out.Spec.Containers[i]
			for j := range c.Files {
				if strings.HasPrefix(c.Files[j].Path, "/etc/mpp/ssh/") {
					c.Files[j].Content = ""
				}
			}
		}
		out.Spec.RunID = ""
		raw, err := json.Marshal(out.Spec)
		if err != nil {
			t.Fatal(err)
		}
		if first == "" {
			first = string(raw)
		} else if string(raw) != first {
			t.Fatal("the same cluster compiled to different RunSpecs")
		}
	}
}
