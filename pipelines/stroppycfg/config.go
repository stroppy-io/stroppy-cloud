// Package stroppycfg is the pure half of running stroppy 6: it renders a
// RunSpec segment into stroppy-config.json and the `stroppy run` command
// line, and reads the bench summary, the error block, the TPC-C compliance
// report and the baseline report back into spec values.
//
// Nothing here touches Docker or Temporal, so the server can use the same
// code to preview a config, and every mapping is unit-tested against the
// output formats of the stroppy checkout (pkg/bench/runtime.go summary,
// pkg/bench/error_reporter.go, workloads/tpcc/report.go,
// cmd/stroppy/commands/baseline).
//
// doc: stroppy `help config-file` (stroppy-config.json envelope),
// `stroppy run --help`, `stroppy baseline --help`.
package stroppycfg

import (
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Layout of the segment workspace inside the stroppy container.
const (
	// ConfigFile is the stroppy config the pipeline writes.
	ConfigFile = "stroppy-config.json"
	// CACertFile is where a workload CA certificate is written.
	CACertFile = "ca.pem"
	// ContainerWorkspace is the segment directory mounted into the
	// container; the image's own WORKDIR is /workspace as well.
	ContainerWorkspace = "/workspace"
)

// Logger enum names stroppy accepts (`stroppy help config-file`).
const (
	logModeProduction = "LOG_MODE_PRODUCTION"
	defaultLogLevel   = "info"
)

// Input is everything the config of one segment depends on.
type Input struct {
	RunID    string
	Segment  spec.Segment
	Workload spec.Workload
	// URL is the connection URL with address placeholders already expanded.
	URL string
	// OTLPEndpoint receives stroppy metrics; empty disables the exporter.
	OTLPEndpoint string
	// OTLPHeaders is the comma-separated key=value list stroppy sends.
	OTLPHeaders string
	// Labels are stamped on the run's metrics as global.metadata.
	Labels map[string]string
}

// Config renders stroppy-config.json of a segment.
//
// doc: stroppy `help config-file` — script, sql, global, drivers, run,
// params, steps, noSteps. `env` is deliberately never written: stroppy 6
// parameters are typed, the legacy env map has no place in a v6 contract.
func Config(in Input) (map[string]any, error) {
	seg := in.Segment
	if seg.Workload.Script == "" {
		return nil, fmt.Errorf("segment %s: no script", seg.Name)
	}
	params, err := Params(seg)
	if err != nil {
		return nil, err
	}
	run, err := RunObject(seg.Run)
	if err != nil {
		return nil, fmt.Errorf("segment %s: %w", seg.Name, err)
	}
	driver, err := Driver(in)
	if err != nil {
		return nil, err
	}
	global := map[string]any{
		"runId":  in.RunID,
		"logger": map[string]any{"logLevel": logLevel(seg), "logMode": logModeProduction},
	}
	if seg.Seed != nil {
		global["seed"] = *seg.Seed
	}
	labels := maps.Clone(in.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	labels["stroppy.segment"] = seg.Name
	global["metadata"] = labels
	if in.OTLPEndpoint != "" {
		global["exporter"] = map[string]any{"name": "otlp", "otlpExport": OTLPExport(in.OTLPEndpoint, in.OTLPHeaders)}
	}
	cfg := map[string]any{
		"version": "1",
		"script":  seg.Workload.Script,
		"global":  global,
		"drivers": map[string]any{"0": driver},
		"run":     run,
	}
	if len(params) > 0 {
		cfg["params"] = params
	}
	if len(seg.Steps) > 0 {
		cfg["steps"] = seg.Steps
	}
	if len(seg.NoSteps) > 0 {
		cfg["noSteps"] = seg.NoSteps
	}
	return cfg, nil
}

var sqlSectionMarker = regexp.MustCompile(`(?m)^\s*--=\s+\S+`)

// Params renders the typed workload parameters: snake_case schema keys
// become stroppy's lowerCamel config keys, nulls are dropped (unset =
// declared default), file references are resolved against the workspace.
//
// doc: `stroppy probe -o json` → params[].config.
func Params(seg spec.Segment) (map[string]any, error) {
	out := map[string]any{}
	keys := make([]string, 0, len(seg.Workload.Params))
	for k := range seg.Workload.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := seg.Workload.Params[k]
		if v == nil {
			continue
		}
		switch v.(type) {
		case string, bool, float64, float32, int, int64, int32, uint, uint64, json.Number:
		default:
			return nil, fmt.Errorf("segment %s: workload.%s must be a scalar; workload parameters belong next to script", seg.Name, k)
		}
		key := lowerCamel(k)
		switch key {
		case "sqlBody":
			body, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("segment %s: sql_body must be a string", seg.Name)
			}
			if !sqlSectionMarker.MatchString(body) {
				v = "--= inline\n" + body
			}
		case "sqlFile", "schemaFile":
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("segment %s: %s must be a string", seg.Name, k)
			}
			if s == "" {
				continue
			}
		}
		out[key] = v
	}
	return out, nil
}

// RunObject renders the `run` object of the config.
//
// doc: `stroppy run <workload> --help` Run parameters — executor, vus,
// iterations, duration, queryTimeout. The bound of the chosen executor is
// mandatory: stroppy itself would run one iteration or zero seconds.
func RunObject(r spec.RunParams) (map[string]any, error) {
	executor := r.Executor
	if executor == "" {
		executor = spec.ExecutorConstantVUs
	}
	out := map[string]any{"executor": executor, "vus": max(r.VUs, 1)}
	switch executor {
	case spec.ExecutorConstantVUs:
		if r.Duration <= 0 {
			return nil, fmt.Errorf("executor %s needs a duration", executor)
		}
		out["duration"] = r.Duration.Std().String()
	case spec.ExecutorSharedIterations:
		if r.Iterations <= 0 {
			return nil, fmt.Errorf("executor %s needs iterations", executor)
		}
		out["iterations"] = r.Iterations
	default:
		return nil, fmt.Errorf("unknown executor %q", executor)
	}
	if r.QueryTimeout > 0 {
		out["queryTimeout"] = r.QueryTimeout.Std().String()
	}
	return out, nil
}

// Driver renders drivers.0: type and URL from the workload, every other key
// as the server rendered it (already lowerCamel), plus caCertFile when the
// workload ships a CA.
//
// doc: stroppy `help drivers` DRIVER OPTIONS.
func Driver(in Input) (map[string]any, error) {
	w := in.Workload
	if w.DriverType == "" {
		return nil, fmt.Errorf("workload: driver_type is required")
	}
	url := in.URL
	if url == "" {
		url = w.URL
	}
	if url == "" {
		return nil, fmt.Errorf("workload: url is required")
	}
	out := map[string]any{"driverType": w.DriverType, "url": url}
	for k, v := range w.Driver {
		switch k {
		case "driverType", "url":
			return nil, fmt.Errorf("workload.driver.%s must be set through workload.driver_type or workload.url", k)
		case "caCertFile":
			if w.CACert != "" {
				return nil, fmt.Errorf("set workload.ca_cert or driver.caCertFile, not both")
			}
		}
		out[k] = v
	}
	if w.CACert != "" {
		out["caCertFile"] = path.Join(ContainerWorkspace, CACertFile)
	}
	return out, nil
}

// OTLPExport maps an endpoint URL to stroppy's OtlpExport: http(s)://…
// goes over OTLP/HTTP, anything else is treated as a gRPC host:port.
//
// doc: stroppy `help config-file` OTLP METRICS.
func OTLPExport(endpoint, headers string) map[string]any {
	out := map[string]any{}
	switch {
	case strings.HasPrefix(endpoint, "http://"):
		out["otlpHttpEndpoint"] = strings.TrimPrefix(endpoint, "http://")
		out["otlpEndpointInsecure"] = true
	case strings.HasPrefix(endpoint, "https://"):
		out["otlpHttpEndpoint"] = strings.TrimPrefix(endpoint, "https://")
	default:
		out["otlpGrpcEndpoint"] = endpoint
	}
	if headers != "" {
		out["otlpHeaders"] = headers
	}
	return out
}

// Args is the container command of a segment: the image's entrypoint is
// the stroppy binary, so this is `run -f <config> [--flag value…]`. Typed
// extra parameters ride as flags (highest precedence) in sorted order.
//
// doc: `stroppy run --help` — Typed parameter flags, Logging.
func Args(seg spec.Segment) []string {
	args := []string{"run", "-f", path.Join(ContainerWorkspace, ConfigFile), "--log-mode", "production", "--log-level", logLevel(seg)}
	keys := make([]string, 0, len(seg.ExtraParams))
	for k := range seg.ExtraParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--"+k+"="+seg.ExtraParams[k])
	}
	return args
}

// BaselineArgs is the container command of the machine self-check.
//
// doc: `stroppy baseline --help`. The report goes to stdout as JSON and is
// not saved under ~/.stroppy (the container is throwaway); the pg-noop
// server is downloaded without a prompt when the image does not embed it.
func BaselineArgs(b spec.Baseline) []string {
	args := []string{"baseline", "--json", "--no-save", "--download", "always"}
	if b.Quick {
		args = append(args, "--quick")
	}
	if len(b.Tiers) > 0 {
		args = append(args, "--tiers", strings.Join(b.Tiers, ","))
	}
	if b.VUs > 0 {
		args = append(args, "--vus", fmt.Sprint(b.VUs))
	}
	if b.Rows > 0 {
		args = append(args, "--rows", fmt.Sprint(b.Rows))
	}
	if b.Duration > 0 {
		args = append(args, "--duration", b.Duration.Std().String())
	}
	return args
}

// Bound is how long a segment may take: its own scenario bound with
// headroom, or a fixed ceiling for iteration-bounded scenarios whose wall
// time stroppy cannot know.
func Bound(seg spec.Segment, margin, iterationsCeiling time.Duration) time.Duration {
	if seg.Timeout > 0 {
		return seg.Timeout.Std()
	}
	if seg.Run.Executor == spec.ExecutorSharedIterations {
		return iterationsCeiling + seg.Warmup.Std()
	}
	d := seg.Run.Duration.Std()
	return d + d/2 + seg.Warmup.Std() + margin
}

func logLevel(seg spec.Segment) string {
	if seg.LogLevel == "" {
		return defaultLogLevel
	}
	return seg.LogLevel
}

// lowerCamel turns a snake_case schema key into stroppy's config key:
// scale_factor → scaleFactor, pg_unlogged → pgUnlogged, ydb_store_mode →
// ydbStoreMode.
func lowerCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

// MarshalConfig is Config as indented JSON.
func MarshalConfig(in Input) ([]byte, error) {
	cfg, err := Config(in)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(cfg, "", "  ")
}
