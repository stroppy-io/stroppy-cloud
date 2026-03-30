package agent

import (
	"fmt"
	"os"
)

const (
	// RemoteBinPath is where the agent binary is installed on target machines.
	RemoteBinPath = "/usr/local/bin/stroppy-agent"
	// RemoteLogPath is the agent log file on target machines.
	RemoteLogPath = "/var/log/stroppy-agent.log"
	// DefaultAgentPort is the port the agent listens on.
	DefaultAgentPort = 9090
)

// Target represents a machine where the agent is running.
// Host becomes known after the VM/container boots and the agent calls back.
type Target struct {
	ID           string `json:"id"`
	Host         string `json:"host"`     // address for server→agent HTTP (may be localhost in host mode)
	InternalHost string `json:"internal"` // container name for container→container communication
	AgentPort    int    `json:"agent_port"`
}

// Addr returns the agent HTTP address.
func (t Target) Addr() string {
	return fmt.Sprintf("http://%s:%d", t.Host, t.AgentPort)
}

// SelfBinaryPath returns the path to the currently running binary.
func SelfBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("agent: resolve self binary: %w", err)
	}
	return exe, nil
}
