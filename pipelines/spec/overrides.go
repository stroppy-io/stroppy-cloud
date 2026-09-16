package spec

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Runtime is a recipe overlay. Raw fields preserve absent versus false/zero/empty.
// Its shape is defined by test.runtime@1; the resulting Run is validated again.
type Runtime struct {
	Workload             json.RawMessage            `json:"workload,omitempty"`
	Machines             map[string]json.RawMessage `json:"machines,omitempty"`
	Containers           []ContainerOverride        `json:"containers,omitempty"`
	AdditionalContainers []Container                `json:"additional_containers,omitempty"`
	HostPrep             []HostPrep                 `json:"host_prep,omitempty"`
	Scrapes              []Scrape                   `json:"scrapes,omitempty"`
	Flows                []Flow                     `json:"flows,omitempty"`
	Network              json.RawMessage            `json:"network,omitempty"`
	Observability        json.RawMessage            `json:"observability,omitempty"`
	ResultExpectations   *[]string                  `json:"result_expectations,omitempty"`
}

// ContainerOverride selects generated containers; both selectors mean intersection.
type ContainerOverride struct {
	Role string          `json:"role,omitempty"`
	Name string          `json:"name,omitempty"`
	Set  json.RawMessage `json:"set"`
}

// Overlay merges objects, replaces scalar/array values, and merges container files
// by path. The returned value never aliases the base. Empty files clears the list.
func Overlay[T any](base T, patch json.RawMessage, out *T) error {
	raw, err := json.Marshal(base)
	if err != nil {
		return err
	}
	dst, err := DecodeObject(raw)
	if err != nil {
		return err
	}
	src, err := DecodeObject(patch)
	if err != nil {
		return err
	}
	if src == nil {
		return fmt.Errorf("override must be an object")
	}
	mergeObject(dst, src)
	raw, err = json.Marshal(dst)
	if err != nil {
		return err
	}
	var next T
	if err := json.Unmarshal(raw, &next); err != nil {
		return err
	}
	*out = next
	return nil
}

func mergeObject(dst, src map[string]any) {
	for k, v := range src {
		if obj, ok := v.(map[string]any); ok && len(obj) > 0 {
			if old, ok := dst[k].(map[string]any); ok {
				mergeObject(old, obj)
				continue
			}
		}
		if k == "files" {
			if files, ok := v.([]any); ok && len(files) > 0 {
				if old, ok := dst[k].([]any); ok {
					v = mergeFiles(old, files)
				}
			}
		}
		dst[k] = v
	}
}

func mergeFiles(base, patch []any) []any {
	out := append([]any(nil), base...)
	for _, v := range patch {
		f, ok := v.(map[string]any)
		if !ok {
			out = append(out, v)
			continue
		}
		found := false
		for i, x := range out {
			if old, ok := x.(map[string]any); ok && old["path"] == f["path"] {
				out[i] = v
				found = true
				break
			}
		}
		if !found {
			out = append(out, v)
		}
	}
	return out
}

// ApplyRuntime applies a validated overlay to a compiled recipe. Final structural
// and capacity validation must run after all overlays, before any cloud action.
func ApplyRuntime(run *Run, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var r Runtime
	if err := json.Unmarshal(raw, &r); err != nil {
		return err
	}
	keys := make([]string, 0, len(r.Machines))
	for key := range r.Machines {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, name := range keys {
		found := false
		for i := range run.Machines {
			if run.Machines[i].Name == name {
				found = true
				if err := Overlay(run.Machines[i], r.Machines[name], &run.Machines[i]); err != nil {
					return err
				}
			}
		}
		if !found {
			return &ValidationError{Issues: []ResourceIssue{{Scope: "input", Path: "machines." + name, Code: "selector", Severity: "ERROR", Message: "no such generated machine"}}}
		}
	}
	for i, p := range r.Containers {
		found := false
		for j, c := range run.Containers {
			if (p.Role != "" || p.Name != "") && (p.Role == "" || p.Role == c.Role) && (p.Name == "" || p.Name == c.Name) {
				found = true
				if err := Overlay(c, p.Set, &run.Containers[j]); err != nil {
					return err
				}
			}
		}
		if !found {
			return &ValidationError{Issues: []ResourceIssue{{Scope: "input", Path: fmt.Sprintf("containers[%d]", i), Code: "selector", Severity: "ERROR", Message: "selector matches no generated container"}}}
		}
	}
	run.Containers = append(run.Containers, r.AdditionalContainers...)
	run.HostPrep = append(run.HostPrep, r.HostPrep...)
	run.Scrapes = append(run.Scrapes, r.Scrapes...)
	run.Flows = append(run.Flows, r.Flows...)
	if len(r.Workload) > 0 {
		if err := Overlay(run.Workload, r.Workload, &run.Workload); err != nil {
			return err
		}
	}
	if len(r.Network) > 0 {
		if err := Overlay(run.Network, r.Network, &run.Network); err != nil {
			return err
		}
	}
	if len(r.Observability) > 0 {
		if err := Overlay(run.Observability, r.Observability, &run.Observability); err != nil {
			return err
		}
	}
	if r.ResultExpectations != nil {
		run.ResultExpectations = *r.ResultExpectations
	}
	return nil
}
