package agent

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
)

// Client sends commands to agents and receives reports.
type Client interface {
	// Send dispatches a command to a single agent and waits for completion.
	Send(nc *dag.NodeContext, target Target, cmd Command) error
	// SendAll dispatches the same command to all targets in parallel, fails on first error.
	SendAll(nc *dag.NodeContext, targets []Target, cmd Command) error
}
