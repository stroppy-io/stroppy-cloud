package run

import (
	"sync"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
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

	// Cloud-specific: terraform working directory ID and actor for teardown.
	terraformWdId  string
	terraformActor *terraform.Actor

	// Effective configs per component, set by config tasks at runtime.
	effectiveConfigs map[string]map[string]string
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

func (s *State) SetTerraformWdId(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.terraformWdId = id
}

func (s *State) TerraformWdId() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.terraformWdId
}

func (s *State) SetTerraformActor(a *terraform.Actor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.terraformActor = a
}

func (s *State) TerraformActor() *terraform.Actor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.terraformActor
}

func (s *State) SetEffectiveConfig(component string, cfg map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.effectiveConfigs == nil {
		s.effectiveConfigs = make(map[string]map[string]string)
	}
	s.effectiveConfigs[component] = cfg
}

// AllTargets returns all known agent targets across all roles.
func (s *State) AllTargets() []agent.Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var all []agent.Target
	all = append(all, s.dbTargets...)
	all = append(all, s.monitorTargets...)
	all = append(all, s.proxyTargets...)
	if s.stroppyTarget != nil {
		all = append(all, *s.stroppyTarget)
	}
	return all
}

// ExportRunState serializes the current state into a dag.RunState for snapshot persistence.
func (s *State) ExportRunState() *dag.RunState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rs := &dag.RunState{
		ContainerIDs:     append([]string{}, s.containerIDs...),
		NetworkID:        s.networkID,
		DBHost:           s.dbHost,
		DBPort:           s.dbPort,
		EffectiveConfigs: s.effectiveConfigs,
	}

	for _, t := range s.dbTargets {
		rs.Targets = append(rs.Targets, dag.TargetInfo{ID: t.ID, Host: t.Host, InternalHost: t.InternalHost, AgentPort: t.AgentPort, Role: "database"})
	}
	for _, t := range s.monitorTargets {
		rs.Targets = append(rs.Targets, dag.TargetInfo{ID: t.ID, Host: t.Host, InternalHost: t.InternalHost, AgentPort: t.AgentPort, Role: "monitor"})
	}
	for _, t := range s.proxyTargets {
		rs.Targets = append(rs.Targets, dag.TargetInfo{ID: t.ID, Host: t.Host, InternalHost: t.InternalHost, AgentPort: t.AgentPort, Role: "proxy"})
	}
	if s.stroppyTarget != nil {
		rs.Targets = append(rs.Targets, dag.TargetInfo{ID: s.stroppyTarget.ID, Host: s.stroppyTarget.Host, InternalHost: s.stroppyTarget.InternalHost, AgentPort: s.stroppyTarget.AgentPort, Role: "stroppy"})
	}

	return rs
}

// ImportRunState restores the state from a persisted dag.RunState.
func (s *State) ImportRunState(rs *dag.RunState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.containerIDs = append([]string{}, rs.ContainerIDs...)
	s.networkID = rs.NetworkID
	s.dbHost = rs.DBHost
	s.dbPort = rs.DBPort
	s.effectiveConfigs = rs.EffectiveConfigs

	s.dbTargets = nil
	s.monitorTargets = nil
	s.proxyTargets = nil
	s.stroppyTarget = nil

	for _, t := range rs.Targets {
		target := agent.Target{ID: t.ID, Host: t.Host, InternalHost: t.InternalHost, AgentPort: t.AgentPort}
		switch t.Role {
		case "database":
			s.dbTargets = append(s.dbTargets, target)
		case "monitor":
			s.monitorTargets = append(s.monitorTargets, target)
		case "proxy":
			s.proxyTargets = append(s.proxyTargets, target)
		case "stroppy":
			s.stroppyTarget = &target
		}
	}
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
