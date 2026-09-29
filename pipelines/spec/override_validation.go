package spec

import (
	"encoding/json"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/system"
)

var (
	machineEngine = compiled(system.Machine)
	runtimeEngine = compiled(system.Runtime)
)

// NormalizeMachineOverride validates optional preset overrides without inventing defaults.
func NormalizeMachineOverride(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out json.RawMessage
	if err := normalize(raw, &out, machineEngine); err != nil {
		return nil, err
	}
	return out, nil
}

// NormalizeRuntime validates the shared advanced deployment form.
func NormalizeRuntime(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out json.RawMessage
	if err := normalize(raw, &out, runtimeEngine); err != nil {
		return nil, err
	}
	return out, nil
}
