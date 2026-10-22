package compile

import (
	"encoding/json"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/workload"
)

// noopValidations are the steps that read back what the load wrote; the
// noop driver stores nothing, so they can only fail.
var noopValidations = []string{"validate_population", "validate_answers"}

// noopSteps skips the validation steps of a segment on the noop driver,
// unless the segment lists its steps itself.
func noopSteps(raw json.RawMessage) (json.RawMessage, error) {
	var segment map[string]any
	if err := json.Unmarshal(raw, &segment); err != nil {
		return nil, err
	}
	if steps, ok := segment["steps"].([]any); ok && len(steps) > 0 {
		return raw, nil
	}
	wl, _ := segment["workload"].(map[string]any) //nolint:errcheck // absent = no script
	script, _ := wl["script"].(string)            //nolint:errcheck // absent = no steps
	known := map[string]bool{}
	for _, s := range workload.Steps(script) {
		known[s] = true
	}
	skip, _ := segment["no_steps"].([]any) //nolint:errcheck // absent = none
	has := map[string]bool{}
	for _, s := range skip {
		if name, ok := s.(string); ok {
			has[name] = true
		}
	}
	changed := false
	for _, v := range noopValidations {
		if known[v] && !has[v] {
			skip = append(skip, v)
			changed = true
		}
	}
	if !changed {
		return raw, nil
	}
	segment["no_steps"] = skip
	return json.Marshal(segment)
}
