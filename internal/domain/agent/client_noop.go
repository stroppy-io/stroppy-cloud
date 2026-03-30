package agent

import (
	"go.uber.org/zap"

	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
)

// NoopClient logs commands but does not execute them.
// Used for testing the DAG flow before the real agent HTTP client is implemented.
type NoopClient struct{}

func (NoopClient) Send(nc *dag.NodeContext, target Target, cmd Command) error {
	nc.Log().Info("noop: would send command",
		zap.String("target", target.ID),
		zap.String("action", string(cmd.Action)),
	)
	return nil
}

func (NoopClient) SendAll(nc *dag.NodeContext, targets []Target, cmd Command) error {
	for _, t := range targets {
		nc.Log().Info("noop: would send command",
			zap.String("target", t.ID),
			zap.String("action", string(cmd.Action)),
		)
	}
	return nil
}
