package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stroppy-io/stroppy/pkg/probe"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
)

// runProbe writes script (and optional sql) to temp files,
// calls stroppy's probe.ScriptInTmp, and maps the result to our ProbeResult proto.
func runProbe(script []byte, sql []byte) (*pb.ProbeResult, error) {
	dir, err := os.MkdirTemp("", "stroppy-probe-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	scriptPath := filepath.Join(dir, "script.ts")
	if err := os.WriteFile(scriptPath, script, 0o644); err != nil {
		return nil, fmt.Errorf("write script: %w", err)
	}

	sqlPath := ""
	if len(sql) > 0 {
		sqlPath = filepath.Join(dir, "queries.sql")
		if err := os.WriteFile(sqlPath, sql, 0o644); err != nil {
			return nil, fmt.Errorf("write sql: %w", err)
		}
	}

	pp, err := probe.ScriptInTmp(scriptPath, sqlPath)
	if err != nil {
		return nil, fmt.Errorf("probe script: %w", err)
	}

	result := &pb.ProbeResult{
		GlobalConfig: pp.GlobalConfig,
		Steps:        pp.Steps,
	}

	// First driver config.
	if len(pp.Drivers) > 0 {
		result.DriverConfig = pp.Drivers[0]
	}

	// K6 options → google.protobuf.Struct via JSON round-trip.
	if pp.Options != nil {
		if optJSON, err := json.Marshal(pp.Options); err == nil {
			s := &structpb.Struct{}
			if err := s.UnmarshalJSON(optJSON); err == nil {
				result.K6Options = s
			}
		}
	}

	// Env params from declarations + raw envs.
	seen := map[string]bool{}
	for _, decl := range pp.EnvDeclarations {
		name := ""
		if len(decl.Names) > 0 {
			name = decl.Names[0]
		}
		if name != "" {
			seen[name] = true
			result.EnvParams = append(result.EnvParams, &pb.EnvParam{
				Name:         name,
				DefaultValue: decl.Default,
				Description:  decl.Description,
			})
		}
	}
	for _, env := range pp.Envs {
		if !seen[env] {
			result.EnvParams = append(result.EnvParams, &pb.EnvParam{Name: env})
		}
	}

	// SQL sections.
	for _, s := range pp.SQLSections {
		result.SqlSections = append(result.SqlSections, s.Name)
	}

	return result, nil
}
