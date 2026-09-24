package spec

import (
	"encoding/json"
	"fmt"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	specschema "github.com/stroppy-io/stroppy-cloud/pipelines/schemas/spec"
)

// NormalizeRun applies the product contract before provisioning even when the
// caller launches through Graphene without the StroppyCloud server.
func NormalizeRun(in Run) (Run, error) {
	var out Run
	err := normalize(in, &out, specschema.Run())
	if err == nil {
		err = ResourceErrors(CheckResources(out))
		if err == nil {
			EnsureDiskPreparation(&out)
		}
	}
	return out, err
}

// NormalizeSuite validates every child before dispatching the first one.
func NormalizeSuite(in Suite) (Suite, error) {
	var out Suite
	err := normalize(in, &out, specschema.Suite())
	if err == nil {
		for _, cell := range out.Cells {
			if e := ResourceErrors(CheckResources(cell.RunSpec)); e != nil {
				return Suite{}, fmt.Errorf("cell %s: %w", cell.ID, e)
			}
		}
	}
	return out, err
}

func normalize(in, out any, schema *schemapb.Schema) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	value, err := DecodeObject(raw)
	if err != nil {
		return err
	}
	e, err := schemapb.Compile(schema)
	if err != nil {
		return err
	}
	_, result, err := e.Bake(value)
	if err != nil {
		return err
	}
	if result.Blocking() {
		var issues []ResourceIssue
		for _, issue := range result.GetErrors() {
			severity := "ERROR"
			if issue.GetSeverity() == schemapb.Schema_Field_SEVERITY_WARNING {
				severity = "WARNING"
			}
			issues = append(issues, ResourceIssue{Scope: "input", Path: issue.GetPath(), Code: issue.GetCode().String(), Severity: severity, Message: issue.GetMessage()})
		}
		// Do not put rejected values (possibly credentials) in workflow errors.
		return &ValidationError{Issues: issues}
	}
	raw, err = json.Marshal(wireValue(value))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// Keep opaque nested JSON values in the public duration wire form as well.
func wireValue(value any) any {
	switch v := value.(type) {
	case time.Duration:
		return v.String()
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, x := range v {
			out[key] = wireValue(x)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = wireValue(x)
		}
		return out
	default:
		return value
	}
}

func NormalizeProviderConfig(in ProviderConfig) (ProviderConfig, error) {
	var out ProviderConfig
	err := normalize(in, &out, specschema.ProviderConfig())
	return out, err
}
