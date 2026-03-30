//go:build integration

// Package tests contains end-to-end integration tests.
// These tests spin up real Docker containers, install databases, and run stroppy.
//
// Run with: go test -tags=integration -timeout 30m -v ./tests/ -run TestE2E
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/api"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// ---------------------------------------------------------------------------
// E2E helpers
// ---------------------------------------------------------------------------

type e2eServer struct {
	app *api.App
	srv *httptest.Server
	url string
}

func startE2E(t *testing.T) *e2eServer {
	t.Helper()

	// Ensure ubuntu:22.04 image is available.
	out, err := exec.Command("docker", "image", "inspect", "ubuntu:22.04").CombinedOutput()
	if err != nil {
		t.Log("pulling ubuntu:22.04...")
		if o, err := exec.Command("docker", "pull", "ubuntu:22.04").CombinedOutput(); err != nil {
			t.Fatalf("docker pull ubuntu:22.04 failed: %s\n%s", err, o)
		}
	}
	_ = out

	// Ensure stroppy-cloud binary is built at /tmp/stroppy-cloud for bind-mount.
	build := exec.Command("go", "build", "-o", "/tmp/stroppy-cloud", "./cmd/cli/")
	build.Dir = ".." // module root
	if o, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build stroppy-cloud binary: %s\n%s", err, o)
	}
	t.Setenv("STROPPY_BINARY_HOST_PATH", "/tmp/stroppy-cloud")

	logger, _ := zap.NewDevelopment()
	app, err := api.New(api.Config{Logger: logger})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	s := api.NewServer(app, logger, "")
	ts := httptest.NewServer(s.Router())

	t.Cleanup(func() {
		// Cleanup any leftover containers.
		cleanup := exec.Command("bash", "-c",
			`docker ps -a --filter "name=stroppy-agent" -q | xargs -r docker rm -f 2>/dev/null; `+
				`docker network rm stroppy-run-net 2>/dev/null; true`)
		cleanup.Run()
		ts.Close()
		app.Close()
	})

	return &e2eServer{app: app, srv: ts, url: ts.URL}
}

func (s *e2eServer) run(t *testing.T, cfg types.RunConfig, expectSuccess bool) {
	t.Helper()

	// Cleanup from previous test.
	exec.Command("bash", "-c",
		`docker ps -a --filter "name=stroppy-agent" -q | xargs -r docker rm -f 2>/dev/null; true`).Run()

	data, _ := json.Marshal(cfg)
	resp, err := http.Post(s.url+"/api/v1/run", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	resp.Body.Close()

	// Poll status until done or timeout.
	deadline := time.Now().Add(5 * time.Minute)
	for {
		if time.Now().After(deadline) {
			t.Fatal("run timed out after 5 minutes")
		}
		time.Sleep(5 * time.Second)

		resp, err := http.Get(s.url + "/api/v1/run/" + cfg.ID + "/status")
		if err != nil {
			t.Logf("status poll error: %v", err)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			// Run might have completed and been cleaned up, or not yet saved.
			resp.Body.Close()

			// Check if containers are still running.
			out, _ := exec.Command("docker", "ps", "--filter", "name=stroppy-agent", "-q").CombinedOutput()
			if len(strings.TrimSpace(string(out))) == 0 {
				// No containers running — run completed (success or failure).
				break
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var snap struct {
			Nodes []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				Error  string `json:"error"`
			} `json:"nodes"`
		}
		json.Unmarshal(body, &snap)

		allDone := true
		hasFailed := false
		for _, n := range snap.Nodes {
			if n.Status == "failed" {
				hasFailed = true
			}
			if n.Status == "pending" {
				allDone = false
			}
		}

		if hasFailed || allDone {
			if hasFailed && expectSuccess {
				t.Logf("snapshot: %s", body)
				for _, n := range snap.Nodes {
					if n.Status == "failed" {
						t.Errorf("node %s failed: %s", n.ID, n.Error)
					}
				}
				t.Fatal("run failed unexpectedly")
			}
			break
		}
	}

	// Verify no containers leaked.
	out, _ := exec.Command("docker", "ps", "-a", "--filter", "name=stroppy-agent", "-q").CombinedOutput()
	containers := strings.TrimSpace(string(out))
	if containers != "" {
		t.Logf("WARNING: leftover containers: %s", containers)
		exec.Command("bash", "-c",
			`docker ps -a --filter "name=stroppy-agent" -q | xargs -r docker rm -f`).Run()
	}
}

func pgRunConfig(id string, preset types.PostgresPreset, version, workload, duration string, workers int) types.RunConfig {
	topo := types.PostgresPresets[preset]
	dbCount := 1
	if len(topo.Replicas) > 0 {
		dbCount += topo.Replicas[0].Count
	}
	machines := []types.MachineSpec{
		{Role: types.RoleDatabase, Count: dbCount, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
		{Role: types.RoleMonitor, Count: 1, CPUs: 1, MemoryMB: 2048, DiskGB: 20},
		{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20},
	}
	if topo.HAProxy != nil {
		machines = append(machines, types.MachineSpec{Role: types.RoleProxy, Count: topo.HAProxy.Count, CPUs: 2, MemoryMB: 2048, DiskGB: 20})
	}
	return types.RunConfig{
		ID:       id,
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.10.0.0/24"},
		Machines: machines,
		Database: types.DatabaseConfig{
			Kind:     types.DatabasePostgres,
			Version:  version,
			Postgres: &topo,
		},
		Monitor: types.MonitorConfig{},
		Stroppy: types.StroppyConfig{
			Version:  "3.1.0",
			Workload: workload,
			Duration: duration,
			Workers:  workers,
		},
	}
}

// ===========================================================================
// Postgres E2E — full pipeline with stroppy run
// ===========================================================================

func TestE2E_Postgres_Single_TPCB(t *testing.T) {
	ts := startE2E(t)
	cfg := pgRunConfig("e2e-pg-s-tpcb", types.PostgresSingle, "16", "tpcb", "10s", 2)
	ts.run(t, cfg, true)
}

func TestE2E_Postgres_Single_Simple(t *testing.T) {
	ts := startE2E(t)
	cfg := pgRunConfig("e2e-pg-s-simple", types.PostgresSingle, "16", "simple", "10s", 2)
	ts.run(t, cfg, true)
}

func TestE2E_Postgres_Single_TPCC(t *testing.T) {
	ts := startE2E(t)
	cfg := pgRunConfig("e2e-pg-s-tpcc", types.PostgresSingle, "16", "tpcc", "10s", 2)
	ts.run(t, cfg, true)
}

func TestE2E_Postgres_Single_Workers(t *testing.T) {
	ts := startE2E(t)
	for _, w := range []int{1, 4, 8} {
		t.Run(fmt.Sprintf("vus-%d", w), func(t *testing.T) {
			cfg := pgRunConfig(fmt.Sprintf("e2e-pg-vus%d", w), types.PostgresSingle, "16", "tpcb", "10s", w)
			ts.run(t, cfg, true)
		})
	}
}

func TestE2E_Postgres_Single_PG17(t *testing.T) {
	ts := startE2E(t)
	cfg := pgRunConfig("e2e-pg17-s-tpcb", types.PostgresSingle, "17", "tpcb", "10s", 2)
	ts.run(t, cfg, true)
}

func TestE2E_Postgres_HA(t *testing.T) {
	ts := startE2E(t)
	// Use HA topology but without etcd (etcd requires cluster formation which is
	// unreliable in ephemeral Docker containers). Streaming replication still works.
	topo := types.PostgresPresets[types.PostgresHA]
	topo.Etcd = false // disable etcd for Docker testing
	cfg := types.RunConfig{
		ID:       "e2e-pg-ha-tpcb",
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.10.0.0/24"},
		Machines: []types.MachineSpec{
			{Role: types.RoleDatabase, Count: 3, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
			{Role: types.RoleProxy, Count: 1, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
			{Role: types.RoleMonitor, Count: 1, CPUs: 1, MemoryMB: 2048, DiskGB: 20},
			{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20},
		},
		Database: types.DatabaseConfig{Kind: types.DatabasePostgres, Version: "16", Postgres: &topo},
		Monitor:  types.MonitorConfig{},
		Stroppy:  types.StroppyConfig{Version: "3.1.0", Workload: "tpcb", Duration: "10s", Workers: 2},
	}
	ts.run(t, cfg, true)
}

func TestE2E_Postgres_Scale(t *testing.T) {
	ts := startE2E(t)
	cfg := pgRunConfig("e2e-pg-scale-tpcb", types.PostgresScale, "16", "tpcb", "10s", 4)
	ts.run(t, cfg, true)
}

func TestE2E_Postgres_StroppyOptions(t *testing.T) {
	ts := startE2E(t)
	cfg := pgRunConfig("e2e-pg-opts", types.PostgresSingle, "16", "tpcb", "10s", 2)
	cfg.Stroppy.Options = map[string]string{
		"SCALE_FACTOR": "1",
	}
	ts.run(t, cfg, true)
}

// MySQL and Picodata E2E tests are in separate files:
// e2e_mysql_test.go, e2e_picodata_test.go
