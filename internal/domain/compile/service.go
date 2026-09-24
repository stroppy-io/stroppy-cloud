package compile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Service is the run.Compiler: it resolves the workload through the
// library, compiles the RunSpec and bakes it through spec.run@1.
type Service struct {
	renderer Renderer
	catalog  *catalog.Catalog
	library  *library.Service
	// Observability is where every run sends telemetry.
	Observability spec.Observability
	// StroppyImage overrides the catalog image of every run (staging).
	StroppyImage string
}

// NewService wires the compiler.
func NewService(r Renderer, cat *catalog.Catalog, lib *library.Service) *Service {
	s := &Service{renderer: r, catalog: cat, library: lib}
	if lib != nil {
		lib.SetPreflight(s.preflight)
	}
	return s
}

// Compile implements run.Compiler.
func (s *Service) Compile(ctx context.Context, req run.CompileRequest) (run.Compiled, error) {
	_, baked, _, err := s.library.DeriveWorkload(ctx, req.Workload)
	if err != nil {
		return run.Compiled{}, err
	}
	prov, ok := s.catalog.Provider(string(req.Profile.Kind))
	if !ok {
		return run.Compiled{}, errs.Invalid(fmt.Sprintf("unknown provider %q", req.Profile.Kind))
	}
	obs := s.Observability
	obs.Labels = map[string]string{"stroppy_run_id": req.RunID.String(), "stroppy_tenant": req.Tenant}
	out, err := Compile(ctx, s.renderer, Input{
		RunID: req.RunID, Tenant: req.Tenant, Database: req.Database, Plan: req.Derived.Plan, EffectiveConfigs: req.Derived.EffectiveConfigs,
		Workload: req.Workload, WorkloadBaked: baked, Sizes: req.Sizes, Execution: req.Execution, Provider: prov, ProviderKind: string(req.Profile.Kind),
		ProviderSettings: req.Profile.Settings, CredentialsSecret: provider.ActiveCredentials(req.Profile), ProviderConfigName: spec.ProviderConfigName(req.Namespace, req.Profile.ID.String()),
		StroppyImage: s.StroppyImage, Keep: req.Keep, Observability: obs, Catalog: s.catalog, Labels: req.Labels,
	})
	if err != nil {
		return run.Compiled{}, err
	}
	raw, err := json.Marshal(out.Spec)
	if err != nil {
		return run.Compiled{}, err
	}
	bakedSpec, err := s.renderer.Bake(ctx, "spec.run@1", raw)
	if err != nil {
		return run.Compiled{}, err
	}
	return run.Compiled{Spec: bakedSpec, Machines: out.Machines}, nil
}

func (s *Service) preflight(ctx context.Context, db library.DatabaseSpec, wl library.WorkloadSpec, res library.Resolved, test library.TestSpec) ([]library.Issue, error) {
	compiled, err := s.Compile(ctx, run.CompileRequest{RunID: uuid.MustParse("00000000-0000-4000-8000-000000000001"), Tenant: "preflight", Namespace: "t-preflight", Database: db, Derived: *res.DatabaseDerived, Workload: wl, Sizes: test.Sizes, Execution: test.Execution, Profile: *res.Profile, Keep: test.Keep})
	if err != nil {
		var v *spec.ValidationError
		if errors.As(err, &v) {
			return preflightIssues(v.Issues), nil
		}
		return nil, err
	}
	var r spec.Run
	if err := json.Unmarshal(compiled.Spec, &r); err != nil {
		return nil, err
	}
	return preflightIssues(spec.CheckResources(r)), nil
}

func preflightIssues(findings []spec.ResourceIssue) []library.Issue {
	var issues []library.Issue
	for _, i := range findings {
		issues = append(issues, library.Issue{Scope: i.Scope, Path: i.Path, Code: i.Code, Severity: i.Severity, Message: i.Message})
	}
	return issues
}
