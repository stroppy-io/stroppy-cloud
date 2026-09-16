package spec

import (
	"encoding/json"
	"fmt"
)

// Segment is a workload.segment@1 value as the pipeline reads it: the
// fields it interprets are typed, the workload parameters stay a generic
// object (their set depends on the script) and the whole value is kept in
// Raw for anything a newer schema adds.
type Segment struct {
	Name string `json:"name"`
	// Workload is the discriminated `workload` object: `script` plus the
	// typed parameters of that script (snake_case, as the schema names them).
	Workload WorkloadParams `json:"workload"`
	Run      RunParams      `json:"run"`
	Steps    []string       `json:"steps,omitempty"`
	NoSteps  []string       `json:"no_steps,omitempty"`
	// ExtraParams are typed stroppy flags by flag name (load-workers) the
	// schema does not model; rendered as `--<name> <value>`.
	ExtraParams map[string]string `json:"extra_params,omitempty"`
	Files       []SegmentFile     `json:"files,omitempty"`
	Thresholds  Thresholds        `json:"thresholds,omitempty,omitzero"`
	Seed        *uint64           `json:"seed,omitempty"`
	Timeout     Duration          `json:"timeout,omitempty"`
	Warmup      Duration          `json:"warmup,omitempty"`
	LogLevel    string            `json:"log_level,omitempty"`
	Raw         json.RawMessage   `json:"-"`
}

// WorkloadParams is the `workload` object of a segment: the script id and
// its parameters. Params holds every key except `script`, values as the
// schema baked them (numbers, bools, strings, durations as "5m").
type WorkloadParams struct {
	Script string
	Params map[string]any
}

// UnmarshalJSON splits the discriminator from the parameters.
func (w *WorkloadParams) UnmarshalJSON(b []byte) error {
	m, err := DecodeObject(b)
	if err != nil {
		return err
	}
	script, ok := m["script"].(string)
	if !ok || script == "" {
		return fmt.Errorf("workload: script is required")
	}
	delete(m, "script")
	w.Script = script
	w.Params = m
	return nil
}

// MarshalJSON puts the discriminator back.
func (w WorkloadParams) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(w.Params)+1)
	for k, v := range w.Params {
		m[k] = v
	}
	m["script"] = w.Script
	return json.Marshal(m)
}

// RunParams is the stroppy scenario: executor, VUs and its bound.
//
// doc: stroppy `run <workload> --help` "Run parameters".
type RunParams struct {
	Executor     string   `json:"executor,omitempty"`
	VUs          int64    `json:"vus,omitempty"`
	Duration     Duration `json:"duration,omitempty"`
	Iterations   int64    `json:"iterations,omitempty"`
	QueryTimeout Duration `json:"query_timeout,omitempty"`
}

// Executors stroppy 6 knows.
const (
	ExecutorConstantVUs      = "constant-vus"
	ExecutorSharedIterations = "shared-iterations"
)

// Thresholds are the pass/fail bounds the pipeline applies to a segment's
// bench summary.
type Thresholds struct {
	P99Ms     float64  `json:"p99_ms,omitempty"`
	ErrorRate *float64 `json:"error_rate,omitempty"`
}

// SegmentFile is a file shipped next to the config.
type SegmentFile struct {
	Name    string `json:"name"`
	Kind    string `json:"kind,omitempty"`
	Content string `json:"content,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

// DecodeSegments parses the workload segments of a run; the raw value is
// kept in Raw, so the pipeline tolerates schema growth.
func DecodeSegments(raw []json.RawMessage) ([]Segment, error) {
	out := make([]Segment, 0, len(raw))
	for i, r := range raw {
		var s Segment
		if err := json.Unmarshal(r, &s); err != nil {
			return nil, fmt.Errorf("segment %d: %w", i, err)
		}
		if s.Name == "" {
			return nil, fmt.Errorf("segment %d: empty name", i)
		}
		if s.Workload.Script == "" {
			return nil, fmt.Errorf("segment %d (%s): no workload script", i, s.Name)
		}
		s.Raw = r
		out = append(out, s)
	}
	return out, nil
}

// MarshalJSON preserves explicitly empty inline files while omitting content
// for an artifact reference, matching the schema's exclusive source rule.
func (f SegmentFile) MarshalJSON() ([]byte, error) {
	type plain SegmentFile
	if f.Ref != "" {
		return json.Marshal(plain(f))
	}
	return json.Marshal(struct {
		plain
		Content string `json:"content"`
	}{plain(f), f.Content})
}
