package api

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/examples"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func exampleOf(e examples.Example) oas.Example {
	out := oas.Example{ID: e.ID, Kind: oas.ExampleKind(e.Kind), Title: e.Title, Document: oas.NewOptExportDocument(*exportDocumentOf(e.Document))}
	if e.Description != "" {
		out.Description = oas.NewOptString(e.Description)
	}
	if e.DBKind != "" {
		out.DbKind = oas.NewOptDatabaseKind(oas.DatabaseKind(e.DBKind))
	}
	if len(e.Tags) > 0 {
		out.Tags = oas.NewOptExampleTags(oas.ExampleTags(e.Tags))
	}
	return out
}

// ListExamples — the gallery (public catalog).
func (h *Handler) ListExamples(ctx context.Context, params oas.ListExamplesParams) (*oas.ListExamplesOK, error) {
	if !h.publicConfig(ctx).Examples {
		return nil, errs.NotFound("examples")
	}
	kind, dbKind := "", ""
	if v, ok := params.Kind.Get(); ok {
		kind = string(v)
	}
	if v, ok := params.DbKind.Get(); ok {
		dbKind = string(v)
	}
	list := h.deps.Examples.List(kind, dbKind)
	out := &oas.ListExamplesOK{Data: make([]oas.Example, 0, len(list))}
	for _, e := range list {
		out.Data = append(out.Data, exampleOf(e))
	}
	return out, nil
}

// CloneExample — copy into the tenant's library.
func (h *Handler) CloneExample(ctx context.Context, req oas.OptCloneExampleReq, params oas.CloneExampleParams) (*oas.CloneExampleCreated, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	name := ""
	if r, ok := req.Get(); ok {
		name = r.Name.Or("")
	}
	created, err := h.deps.Examples.Clone(ctx, a, t.ID, params.ExampleId, name)
	if err != nil {
		return nil, err
	}
	return &oas.CloneExampleCreated{Created: usagesOf(created)}, nil
}

// QuickRunExample — launch an example test without cloning; the overrides
// carry the provider profile (examples have none).
func (h *Handler) QuickRunExample(ctx context.Context, req *oas.LaunchOverrides, params oas.QuickRunExampleParams) (*oas.Run, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	spec, name, err := h.deps.Examples.TestSpec(ctx, t.ID, params.ExampleId)
	if err != nil {
		return nil, err
	}
	o, err := overridesOf(oas.NewOptLaunchOverrides(*req))
	if err != nil {
		return nil, err
	}
	if o.Name == "" {
		o.Name = name
	}
	if o.ProviderProfileID == nil && spec.ProviderProfileID == nil {
		return nil, errs.Invalid("provider_profile_id is required for a quick run").Because("quick_run_needs_profile")
	}
	r, err := h.deps.Runs.LaunchSpec(ctx, a, t.ID, spec, o, params.IdempotencyKey.Or(""))
	if err != nil {
		return nil, err
	}
	return h.runOf(r, nil), nil
}
