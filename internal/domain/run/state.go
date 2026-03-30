package run

import (
	"sync"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/agent"
)

// State is the shared mutable state for a single run.
// It is populated progressively as DAG phases complete.
// All methods are safe for concurrent use.
type State struct {
	mu sync.RWMutex

	// Populated by the "machines" phase.
	dbTargets      []agent.Target
	monitorTargets []agent.Target
	proxyTargets   []agent.Target
	stroppyTarget  *agent.Target

	// Populated by the "configure_db" phase.
	dbHost string
	dbPort int

	// Docker-specific: container IDs for teardown.
	containerIDs []string
	networkID    string
}

func NewState() *State { return &State{} }

// --- writers (called by tasks) ---

func (s *State) SetDBTargets(targets []agent.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dbTargets = targets
}

func (s *State) SetMonitorTargets(targets []agent.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.monitorTargets = targets
}

func (s *State) SetProxyTargets(targets []agent.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proxyTargets = targets
}

func (s *State) SetStroppyTarget(target agent.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stroppyTarget = &target
}

func (s *State) SetDBEndpoint(host string, port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dbHost = host
	s.dbPort = port
}

// --- readers (called by tasks) ---

func (s *State) DBTargets() []agent.Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbTargets
}

func (s *State) MonitorTargets() []agent.Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.monitorTargets
}

func (s *State) ProxyTargets() []agent.Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.proxyTargets
}

func (s *State) StroppyTarget() *agent.Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stroppyTarget
}

func (s *State) DBEndpoint() (string, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbHost, s.dbPort
}

func (s *State) AddContainerID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containerIDs = append(s.containerIDs, id)
}

func (s *State) ContainerIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.containerIDs...)
}

func (s *State) SetNetworkID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.networkID = id
}

func (s *State) NetworkID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.networkID
}

// AllTargetHosts returns all known hosts for monitoring scrape targets.
func (s *State) AllTargetHosts() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var hosts []string
	for _, t := range s.dbTargets {
		hosts = append(hosts, t.Host)
	}
	for _, t := range s.monitorTargets {
		hosts = append(hosts, t.Host)
	}
	if s.stroppyTarget != nil {
		hosts = append(hosts, s.stroppyTarget.Host)
	}
	return hosts
}
