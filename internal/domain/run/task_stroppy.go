package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type stroppyInstallTask struct {
	client  agent.Client
	state   *State
	stroppy types.StroppyConfig
}

func (t *stroppyInstallTask) Execute(nc *dag.NodeContext) error {
	target := t.state.StroppyTarget()
	if target == nil {
		return fmt.Errorf("stroppy target not provisioned")
	}
	nc.Log().Info("installing stroppy")
	return t.client.Send(nc, *target, agent.Command{
		Action: agent.ActionInstallStroppy,
		Config: agent.StroppyInstallConfig{Version: t.stroppy.Version},
	})
}

type stroppyRunTask struct {
	client          agent.Client
	state           *State
	stroppy         types.StroppyConfig
	stroppySettings types.StroppySettings
	dbKind          types.DatabaseKind
	runID           string
}

func (t *stroppyRunTask) Execute(nc *dag.NodeContext) error {
	target := t.state.StroppyTarget()
	if target == nil {
		return fmt.Errorf("stroppy target not provisioned")
	}
	dbHost, dbPort := t.state.DBEndpoint()
	nc.Log().Info("running stroppy test")

	otlpEnv := t.stroppySettings.StroppyEnv(t.runID)

	return t.client.Send(nc, *target, agent.Command{
		Action: agent.ActionRunStroppy,
		Config: agent.StroppyRunConfig{
			DBHost:   dbHost,
			DBPort:   dbPort,
			DBKind:   string(t.dbKind),
			Workload: t.stroppy.Workload,
			Duration: t.stroppy.Duration,
			Workers:  t.stroppy.Workers,
			Options:  t.stroppy.Options,
			OTLPEnv:  otlpEnv,
		},
	})
}
