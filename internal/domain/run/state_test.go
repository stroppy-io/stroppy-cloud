package run

import (
	"sync"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
)

func TestNewState(t *testing.T) {
	s := NewState()
	if s == nil {
		t.Fatal("NewState() returned nil")
	}
}

func TestState_DBTargets(t *testing.T) {
	s := NewState()
	targets := []agent.Target{
		{ID: "db1", Host: "10.0.0.1", AgentPort: 8080},
		{ID: "db2", Host: "10.0.0.2", AgentPort: 8080},
	}
	s.SetDBTargets(targets)

	got := s.DBTargets()
	if len(got) != 2 {
		t.Fatalf("expected 2 DB targets, got %d", len(got))
	}
	if got[0].Host != "10.0.0.1" {
		t.Errorf("expected host 10.0.0.1, got %s", got[0].Host)
	}
}

func TestState_MonitorTargets(t *testing.T) {
	s := NewState()
	targets := []agent.Target{{ID: "mon1", Host: "10.0.0.10", AgentPort: 9090}}
	s.SetMonitorTargets(targets)

	got := s.MonitorTargets()
	if len(got) != 1 || got[0].ID != "mon1" {
		t.Fatalf("unexpected monitor targets: %+v", got)
	}
}

func TestState_ProxyTargets(t *testing.T) {
	s := NewState()
	s.SetProxyTargets([]agent.Target{{ID: "proxy1", Host: "10.0.0.20"}})

	got := s.ProxyTargets()
	if len(got) != 1 || got[0].ID != "proxy1" {
		t.Fatalf("unexpected proxy targets: %+v", got)
	}
}

func TestState_StroppyTarget(t *testing.T) {
	s := NewState()
	if s.StroppyTarget() != nil {
		t.Fatal("expected nil stroppy target initially")
	}

	s.SetStroppyTarget(agent.Target{ID: "stroppy1", Host: "10.0.0.30"})

	got := s.StroppyTarget()
	if got == nil || got.ID != "stroppy1" {
		t.Fatalf("unexpected stroppy target: %+v", got)
	}
}

func TestState_DBEndpoint(t *testing.T) {
	s := NewState()
	s.SetDBEndpoint("db.local", 5432)

	host, port := s.DBEndpoint()
	if host != "db.local" || port != 5432 {
		t.Fatalf("expected db.local:5432, got %s:%d", host, port)
	}
}

func TestState_ContainerIDs(t *testing.T) {
	s := NewState()
	s.AddContainerID("c1")
	s.AddContainerID("c2")

	ids := s.ContainerIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 container IDs, got %d", len(ids))
	}

	// Verify returned slice is a copy.
	ids[0] = "modified"
	orig := s.ContainerIDs()
	if orig[0] == "modified" {
		t.Fatal("ContainerIDs() should return a copy")
	}
}

func TestState_NetworkID(t *testing.T) {
	s := NewState()
	s.SetNetworkID("net-123")
	if got := s.NetworkID(); got != "net-123" {
		t.Fatalf("expected net-123, got %s", got)
	}
}

func TestState_TerraformWdId(t *testing.T) {
	s := NewState()
	s.SetTerraformWdId("tf-abc")
	if got := s.TerraformWdId(); got != "tf-abc" {
		t.Fatalf("expected tf-abc, got %s", got)
	}
}

func TestState_AllTargetHosts(t *testing.T) {
	s := NewState()
	s.SetDBTargets([]agent.Target{{Host: "db1"}, {Host: "db2"}})
	s.SetMonitorTargets([]agent.Target{{Host: "mon1"}})
	s.SetStroppyTarget(agent.Target{Host: "stroppy1"})

	hosts := s.AllTargetHosts()
	if len(hosts) != 4 {
		t.Fatalf("expected 4 hosts, got %d: %v", len(hosts), hosts)
	}
}

func TestState_ConcurrentAccess(t *testing.T) {
	s := NewState()
	var wg sync.WaitGroup

	// Run concurrent writes and reads.
	for i := 0; i < 100; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			s.SetDBTargets([]agent.Target{{ID: "db", Host: "h"}})
		}()
		go func() {
			defer wg.Done()
			_ = s.DBTargets()
		}()
		go func() {
			defer wg.Done()
			s.SetDBEndpoint("h", 5432)
		}()
		go func() {
			defer wg.Done()
			_, _ = s.DBEndpoint()
		}()
	}

	wg.Wait()
	// If we get here without a race detector panic, the test passes.
}
