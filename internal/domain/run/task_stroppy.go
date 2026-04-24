package run

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

// Sentinel tokens rendered in dry-run previews of the stroppy config. The real
// DB endpoint is only known at execution time; the preview embeds these tokens
// so that user-edited overrides can still be substituted before being sent to
// the stroppy binary.
const (
	DBHostPlaceholder = "__STROPPY_DB_HOST__"
	DBPortPlaceholder = "__STROPPY_DB_PORT__"
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
	nc.Log().Info(fmt.Sprintf("running stroppy test, db_endpoint=%s:%d", dbHost, dbPort))

	settings := t.stroppySettings
	if settings.OTLPEndpoint == "" && t.monitoringURL != "" {
		settings.SetFromMonitoringURL(t.monitoringURL, t.monitoringToken, t.accountID)
	}

	// If the user provided an override: substitute DB endpoint sentinels with the
	// real host/port, then inject tenant OTLP settings if the override doesn't
	// already carry an exporter. This keeps user edits (env, steps, k6_args, etc.)
	// while preserving automatic metrics export.
	if t.stroppy.ConfigOverrideJSON != "" {
		nc.Log().Info("using user-provided stroppy config override")
		cfgJSON := t.stroppy.ConfigOverrideJSON
		cfgJSON = strings.ReplaceAll(cfgJSON, DBHostPlaceholder, dbHost)
		cfgJSON = strings.ReplaceAll(cfgJSON, DBPortPlaceholder, strconv.Itoa(dbPort))

		var rc stroppypb.RunConfig
		if err := protojson.Unmarshal([]byte(cfgJSON), &rc); err != nil {
			return fmt.Errorf("parse stroppy config override: %w", err)
		}
		injectOTLP(&rc, settings, t.runID)
		patched, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(&rc)
		if err != nil {
			return fmt.Errorf("marshal patched stroppy config: %w", err)
		}

		return t.client.Send(nc, *target, agent.Command{
			Action: agent.ActionRunStroppy,
			Config: agent.StroppyRunConfig{ConfigJSON: string(patched)},
		})
	}

	jsonBytes, err := BuildStroppyConfigJSON(t.stroppy, t.dbKind, dbHost, dbPort, settings, t.runID)
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

// injectOTLP populates the stroppy global exporter + OTEL_RESOURCE_ATTRIBUTES
// env var from tenant OTLP settings. Skipped when settings carry no endpoint
// or when the config already defines an OTLP exporter (user edits win).
func injectOTLP(rc *stroppypb.RunConfig, settings types.StroppySettings, runID string) {
	if settings.OTLPEndpoint == "" {
		return
	}
	if rc.Global == nil {
		rc.Global = &stroppypb.GlobalConfig{
			Logger: &stroppypb.LoggerConfig{LogLevel: stroppypb.LoggerConfig_LOG_LEVEL_INFO},
		}
	}
	if rc.Global.Exporter == nil || rc.Global.Exporter.OtlpExport == nil {
		insecure := settings.OTLPInsecure
		endpoint := settings.OTLPEndpoint
		urlPath := settings.OTLPURLPath
		metricPrefix := settings.OTLPMetricPrefix
		otlpExport := &stroppypb.OtlpExport{
			OtlpHttpEndpoint:        &endpoint,
			OtlpHttpExporterUrlPath: &urlPath,
			OtlpEndpointInsecure:    &insecure,
			OtlpMetricsPrefix:       &metricPrefix,
		}
		if settings.OTLPHeaders != "" {
			otlpExport.OtlpHeaders = &settings.OTLPHeaders
		}
		rc.Global.Exporter = &stroppypb.ExporterConfig{OtlpExport: otlpExport}
	}
	if rc.Env == nil {
		rc.Env = map[string]string{}
	}
	if _, ok := rc.Env["OTEL_RESOURCE_ATTRIBUTES"]; !ok {
		svcName := settings.OTLPServiceName
		if svcName == "" {
			svcName = "stroppy"
		}
		rc.Env["OTEL_RESOURCE_ATTRIBUTES"] = fmt.Sprintf("service.name=%s,stroppy.run.id=%s", svcName, runID)
	}
}

// dbDriverURL formats the per-kind connection URL used by the stroppy driver.
// host/port are strings so dry-run can pass sentinel tokens.
func dbDriverURL(dbKind types.DatabaseKind, host, port string) (string, string) {
	switch dbKind {
	case types.DatabasePostgres:
		return fmt.Sprintf("postgresql://postgres@%s:%s/postgres?sslmode=disable", host, port), "postgres"
	case types.DatabaseMySQL:
		return fmt.Sprintf("root@tcp(%s:%s)/", host, port), "mysql"
	case types.DatabasePicodata:
		return fmt.Sprintf("postgres://admin:T0psecret@%s:%s?sslmode=disable", host, port), "picodata"
	case types.DatabaseYDB:
		return fmt.Sprintf("grpc://%s:%s/Root/testdb", host, port), "ydb"
	default:
		return fmt.Sprintf("%s:%s", host, port), string(dbKind)
	}
}

// BuildStroppyConfigJSON generates the protojson config sent to the stroppy binary.
// Exported so dry-run can show users what config will be applied.
// When dbHost=="" and dbPort==0, the generated config embeds sentinel tokens
// (DBHostPlaceholder/DBPortPlaceholder) that stroppyRunTask substitutes at run time.
func BuildStroppyConfigJSON(s types.StroppyConfig, dbKind types.DatabaseKind, dbHost string, dbPort int, settings types.StroppySettings, runID string) ([]byte, error) {
	script := s.Script
	if script == "" {
		script = s.Workload
	}
	if script == "" {
		script = "tpcc/procs"
	}

	hostTok, portTok := dbHost, strconv.Itoa(dbPort)
	if dbHost == "" && dbPort == 0 {
		hostTok, portTok = DBHostPlaceholder, DBPortPlaceholder
	}
	driverURL, driverType := dbDriverURL(dbKind, hostTok, portTok)

	vus := s.VUs
	if vus == 0 && s.VUSScale > 0 {
		vus = int(s.VUSScale)
	}
	if vus == 0 && s.Workers > 0 {
		vus = s.Workers
	}
	if vus == 0 {
		vus = 1
	}

	poolSize := s.PoolSize
	if poolSize == 0 {
		poolSize = 100
	}
	scaleFactor := s.ScaleFactor
	if scaleFactor == 0 {
		scaleFactor = 1
	}
	duration := s.Duration
	if duration == "" {
		duration = "60s"
	}

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
		Steps:   s.Steps,
		NoSteps: s.NoSteps,
		Global: &stroppypb.GlobalConfig{
			Logger: &stroppypb.LoggerConfig{LogLevel: stroppypb.LoggerConfig_LOG_LEVEL_INFO},
		},
	}

	injectOTLP(rc, settings, runID)

	return protojson.MarshalOptions{
		Multiline:     true,
		Indent:        "  ",
		UseProtoNames: false,
	}.Marshal(rc)
}
