package agent

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// StroppyInstallConfig is the agent payload for stroppy installation.
type StroppyInstallConfig struct {
	Version string `json:"version"`
}

// StroppyRunConfig is the agent payload for stroppy test execution.
type StroppyRunConfig struct {
	DBHost   string            `json:"db_host"`
	DBPort   int               `json:"db_port"`
	DBKind   string            `json:"db_kind"`
	Workload string            `json:"workload"`
	Duration string            `json:"duration"`
	Workers  int               `json:"workers"`
	Options  map[string]string `json:"options,omitempty"`
}

// --- DAG tasks ---

type stroppyInstallTask struct {
	client  Client
	target  Target
	stroppy types.StroppyConfig
}

func (t *stroppyInstallTask) Execute(nc *dag.NodeContext) error {
	cfg := StroppyInstallConfig{
		Version: t.stroppy.Version,
	}
	return t.client.Send(nc, t.target, Command{
		Action: ActionInstallStroppy,
		Config: cfg,
	})
}

type stroppyRunTask struct {
	client  Client
	target  Target
	stroppy types.StroppyConfig
	dbHost  string
	dbPort  int
	dbKind  types.DatabaseKind
}

func (t *stroppyRunTask) Execute(nc *dag.NodeContext) error {
	cfg := StroppyRunConfig{
		DBHost:   t.dbHost,
		DBPort:   t.dbPort,
		DBKind:   string(t.dbKind),
		Workload: t.stroppy.Workload,
		Duration: t.stroppy.Duration,
		Workers:  t.stroppy.Workers,
		Options:  t.stroppy.Options,
	}
	return t.client.Send(nc, t.target, Command{
		Action: ActionRunStroppy,
		Config: cfg,
	})
}
