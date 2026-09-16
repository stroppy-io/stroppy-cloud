package stroppycfg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	workloadschema "github.com/stroppy-io/stroppy-cloud/pipelines/schemas/workload"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

var (
	fileName    = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*(/[A-Za-z0-9_-][A-Za-z0-9._-]*)*$`)
	artifactRef = regexp.MustCompile(`^artifact/[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
)

// ValidateFiles protects runtime files and preserves relative subdirectories.
func ValidateFiles(files []spec.SegmentFile) error {
	seen := map[string]bool{}
	for _, f := range files {
		base, _, _ := strings.Cut(f.Name, "/")
		if !fileName.MatchString(f.Name) || base == ConfigFile || base == CACertFile || base == "stroppy.log" || seen[f.Name] {
			return fmt.Errorf("invalid, reserved or duplicate segment file name %q", f.Name)
		}
		for name := range seen {
			if strings.HasPrefix(name, f.Name+"/") || strings.HasPrefix(f.Name, name+"/") {
				return fmt.Errorf("segment file paths overlap: %s and %s", name, f.Name)
			}
		}
		seen[f.Name] = true
		if f.Ref != "" && (!artifactRef.MatchString(f.Ref) || f.Content != "") {
			return fmt.Errorf("segment file %s requires either inline content or artifact/<name>", f.Name)
		}
	}
	return nil
}

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
