package run

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
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
	monitoringURL   string
	monitoringToken string
	accountID       int32
}

func (t *stroppyRunTask) Execute(nc *dag.NodeContext) error {
	target := t.state.StroppyTarget()
	if target == nil {
		return fmt.Errorf("stroppy target not provisioned")
	}
	dbHost, dbPort := t.state.DBEndpoint()
	nc.Log().Info("running stroppy test")

	// Resolve script: new field takes priority, fall back to deprecated Workload.
	script := t.stroppy.Script
	if script == "" {
		script = t.stroppy.Workload
	}
	if script == "" {
		script = "tpcc/procs"
	}

	// Build driver URL.
	var driverURL, driverType string
	switch t.dbKind {
	case types.DatabasePostgres:
		driverURL = fmt.Sprintf("postgresql://postgres@%s:%d/postgres?sslmode=disable", dbHost, dbPort)
		driverType = "postgres"
	case types.DatabaseMySQL:
		driverURL = fmt.Sprintf("root@tcp(%s:%d)/", dbHost, dbPort)
		driverType = "mysql"
	case types.DatabasePicodata:
		driverURL = fmt.Sprintf("postgresql://admin@%s:%d/admin?sslmode=disable", dbHost, dbPort)
		driverType = "picodata"
	default:
		driverURL = fmt.Sprintf("%s:%d", dbHost, dbPort)
		driverType = string(t.dbKind)
	}

	// Resolve VUs: new field, then deprecated VUSScale, then deprecated Workers.
	vus := t.stroppy.VUs
	if vus == 0 && t.stroppy.VUSScale > 0 {
		vus = int(t.stroppy.VUSScale)
	}
	if vus == 0 && t.stroppy.Workers > 0 {
		vus = t.stroppy.Workers
	}
	if vus == 0 {
		vus = 1
	}

	poolSize := t.stroppy.PoolSize
	if poolSize == 0 {
		poolSize = 100
	}
	scaleFactor := t.stroppy.ScaleFactor
	if scaleFactor == 0 {
		scaleFactor = 1
	}
	duration := t.stroppy.Duration
	if duration == "" {
		duration = "60s"
	}

	// Build stroppy RunConfig protobuf.
	maxConns := int32(poolSize)
	rc := &stroppypb.RunConfig{
		Version: "1",
		Script:  &script,
		Drivers: map[uint32]*stroppypb.DriverRunConfig{
			0: {
				DriverType: driverType,
				Url:        driverURL,
				Pool: &stroppypb.DriverRunConfig_PoolConfig{
					MaxConns: &maxConns,
					MinConns: &maxConns,
				},
			},
		},
		Env: map[string]string{
			"SCALE_FACTOR": fmt.Sprintf("%d", scaleFactor),
			"POOL_SIZE":    fmt.Sprintf("%d", poolSize),
		},
		K6Args:  []string{"--vus", fmt.Sprintf("%d", vus), "--duration", duration},
		Steps:   t.stroppy.Steps,
		NoSteps: t.stroppy.NoSteps,
	}

	// OTLP exporter — if monitoring is configured.
	settings := t.stroppySettings
	if settings.OTLPEndpoint == "" && t.monitoringURL != "" {
		settings.SetFromMonitoringURL(t.monitoringURL, t.monitoringToken, t.accountID)
	}
	if settings.OTLPEndpoint != "" {
		insecure := settings.OTLPInsecure
		endpoint := settings.OTLPEndpoint
		rc.Global = &stroppypb.GlobalConfig{
			Exporter: &stroppypb.ExporterConfig{
				OtlpExport: &stroppypb.OtlpExport{
					OtlpGrpcEndpoint:     &endpoint,
					OtlpEndpointInsecure: &insecure,
				},
			},
		}
		// stroppy v4 auto-enables OTLP output when global.exporter is set.
		// No need for --out opentelemetry in k6Args.

		// Set OTEL resource attributes for run correlation.
		runPrefix := t.runID
		rc.Env["OTEL_RESOURCE_ATTRIBUTES"] = fmt.Sprintf("service.name=stroppy,stroppy.run.id=%s", runPrefix)
		if settings.OTLPHeaders != "" {
			rc.Env["K6_OTEL_HEADERS"] = settings.OTLPHeaders
		}
	}

	// Serialize to JSON via protojson (camelCase field names as stroppy expects).
	jsonBytes, err := protojson.MarshalOptions{
		Multiline:     true,
		Indent:        "  ",
		UseProtoNames: false, // camelCase
	}.Marshal(rc)
	if err != nil {
		return fmt.Errorf("marshal stroppy config: %w", err)
	}

	return t.client.Send(nc, *target, agent.Command{
		Action: agent.ActionRunStroppy,
		Config: agent.StroppyRunConfig{
			ConfigJSON: string(jsonBytes),
		},
	})
}
