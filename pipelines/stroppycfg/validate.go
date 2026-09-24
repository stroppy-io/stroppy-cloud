package stroppycfg

import (
	"encoding/json"
	"fmt"
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	workloadschema "github.com/stroppy-io/stroppy-cloud/pipelines/schemas/workload"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// ValidateSegment checks public input semantics before any cloud resource exists.
func ValidateSegment(raw json.RawMessage) error {
	engine, err := schemapb.Compile(workloadschema.Segment())
	if err != nil {
		return err
	}
	value, err := spec.DecodeObject(raw)
	if err != nil {
		return err
	}
	_, result, err := engine.Bake(value)
	if err != nil {
		return err
	}
	if result.Blocking() {
		var issues []string
		for _, issue := range result.GetErrors() {
			issues = append(issues, issue.GetPath()+": "+issue.GetMessage())
		}
		return fmt.Errorf("invalid workload segment: %s", strings.Join(issues, "; "))
	}
	return nil
}
