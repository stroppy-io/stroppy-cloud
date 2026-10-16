package api

import (
	"context"
	"encoding/json"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func segmentsFrom(in []oas.SchemaValue) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(in))
	for _, s := range in {
		out = append(out, rawOf(s))
	}
	return out
}

func segmentsTo(in []json.RawMessage) []oas.SchemaValue {
	out := make([]oas.SchemaValue, 0, len(in))
	for _, s := range in {
		out = append(out, schemaValueOf(s))
	}
	return out
}

func workloadSpecFrom(version string, protocol oas.Protocol, segments []oas.SchemaValue, options oas.OptSchemaValue) library.WorkloadSpec {
	spec := library.WorkloadSpec{StroppyVersion: version, Protocol: catalog.Protocol(protocol), Segments: segmentsFrom(segments)}
	if o, ok := options.Get(); ok {
		spec.Options = rawOf(o)
	}
	return spec
}

func workloadSpecOf(s oas.WorkloadSpec) library.WorkloadSpec {
	return workloadSpecFrom(s.StroppyVersion, s.Protocol, s.Segments, s.Options)
}

func workloadSpecToWire(spec library.WorkloadSpec) oas.WorkloadSpec {
	out := oas.WorkloadSpec{StroppyVersion: spec.StroppyVersion, Protocol: oas.Protocol(spec.Protocol), Segments: segmentsTo(spec.Segments), Schema: oas.NewOptSchemaRef(oas.SchemaRef{ID: "workload.stroppy", Version: "1"})}
	if len(spec.Options) > 0 {
		out.Options = oas.NewOptSchemaValue(schemaValueOf(spec.Options))
	}
	return out
}

func (h *Handler) workloadOf(w library.Workload, derived library.WorkloadDerived, usages []library.Usage) *oas.Workload {
	hd := entityHeader(w.Entity)
	spec := workloadSpecToWire(w.Spec)
	out := &oas.Workload{
		ID: hd.id, Name: hd.name, Description: hd.description, Tags: oas.NewOptWorkloadTags(tagsOf(w.Tags)), Author: hd.author, CreatedAt: hd.created, UpdatedAt: hd.updated,
		StroppyVersion: spec.StroppyVersion, Protocol: spec.Protocol, Segments: spec.Segments, Options: spec.Options, Schema: spec.Schema, Usages: usagesOf(usages),
	}
	if derived.Validation != nil {
		out.Validation = validationErrOf(derived.Validation)
		return out
	}
	out.Requirements = oas.NewOptRequirements(oas.Requirements{"runner": requirementOf(derived.Runner)})
	out.Validation = oas.NewOptValidationResult(oas.ValidationResult{Errors: []oas.ValidationError{}})
	return out
}

func segmentsSummaryOf(segs []library.SegmentSummary) []oas.WorkloadPreviewSegmentsSummaryItem {
	out := make([]oas.WorkloadPreviewSegmentsSummaryItem, 0, len(segs))
	for _, s := range segs {
		item := oas.WorkloadPreviewSegmentsSummaryItem{Name: oas.NewOptString(s.Name), Script: oas.NewOptString(s.Script), Steps: s.Steps}
		if s.VUs > 0 {
			item.Vus = oas.NewOptInt(s.VUs)
		}
		if s.Limit != "" {
			item.Limit = oas.NewOptString(s.Limit)
		}
		if item.Steps == nil {
			item.Steps = []string{}
		}
		out = append(out, item)
	}
	return out
}

// ListWorkloads — definitions of the tenant.
func (h *Handler) ListWorkloads(ctx context.Context, params oas.ListWorkloadsParams) (*oas.ListWorkloadsOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	sortKey, order := "", ""
	if v, ok := params.Sort.Get(); ok {
		sortKey = string(v)
	}
	if v, ok := params.Order.Get(); ok {
		order = string(v)
	}
	q, limit, err := listQueryOf(params.Search, params.Tags, params.Author, sortKey, order, params.Cursor, params.Limit)
	if err != nil {
		return nil, err
	}
	for _, p := range params.Protocol {
		q.Protocols = append(q.Protocols, string(p))
	}
	q.Versions = params.StroppyVersion
	q.Script = params.Script.Or("")
	list, err := h.deps.Library.ListWorkloads(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, q.Offset, limit)
	out := &oas.ListWorkloadsOK{Data: make([]oas.Workload, 0, len(list)), Meta: meta}
	for _, w := range list {
		_, _, derived, derr := h.deps.Library.DeriveWorkload(ctx, w.Spec)
		if derr != nil {
			derived.Validation = derr
		}
		out.Data = append(out.Data, *h.workloadOf(w, derived, nil))
	}
	return out, nil
}

// CreateWorkload — store a definition.
func (h *Handler) CreateWorkload(ctx context.Context, req *oas.WorkloadWrite, params oas.CreateWorkloadParams) (*oas.Workload, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	w := library.EntityWrite{Name: req.Name, Description: req.Description.Or("")}
	if tags, ok := req.Tags.Get(); ok {
		w.Tags = tags
	}
	wl, derived, err := h.deps.Library.CreateWorkload(ctx, a, t.ID, w, workloadSpecFrom(req.StroppyVersion, req.Protocol, req.Segments, req.Options))
	if err != nil {
		return nil, err
	}
	return h.workloadOf(wl, derived, nil), nil
}

// GetWorkload — one definition.
func (h *Handler) GetWorkload(ctx context.Context, params oas.GetWorkloadParams) (*oas.Workload, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	w, derived, usages, err := h.deps.Library.GetWorkload(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.workloadOf(w, derived, usages), nil
}

// PatchWorkload — update.
func (h *Handler) PatchWorkload(ctx context.Context, req *oas.WorkloadPatch, params oas.PatchWorkloadParams) (*oas.Workload, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p := library.EntityPatch{}
	if v, ok := req.Name.Get(); ok {
		p.Name = &v
	}
	if v, ok := req.Description.Get(); ok {
		p.Description = &v
	}
	if v, ok := req.Tags.Get(); ok {
		p.Tags = v
	}
	var spec *library.WorkloadSpec
	if req.StroppyVersion.Set || req.Protocol.Set || req.Segments != nil || req.Options.Set {
		spec = &library.WorkloadSpec{StroppyVersion: req.StroppyVersion.Or("")}
		if v, ok := req.Protocol.Get(); ok {
			spec.Protocol = catalog.Protocol(v)
		}
		if req.Segments != nil {
			spec.Segments = segmentsFrom(req.Segments)
		}
		if o, ok := req.Options.Get(); ok {
			spec.Options = rawOf(o)
		}
	}
	w, derived, err := h.deps.Library.UpdateWorkload(ctx, a, t.ID, params.ID, p, spec)
	if err != nil {
		return nil, err
	}
	return h.workloadOf(w, derived, nil), nil
}

// DeleteWorkload — remove.
func (h *Handler) DeleteWorkload(ctx context.Context, params oas.DeleteWorkloadParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Library.DeleteWorkload(ctx, a, t.ID, params.ID, params.InlineUsages.Or(false))
}

// PreviewWorkload — validate an unsaved definition.
func (h *Handler) PreviewWorkload(ctx context.Context, req *oas.WorkloadWrite, params oas.PreviewWorkloadParams) (*oas.WorkloadPreview, error) {
	if _, _, err := h.tenantOf(ctx, params.Slug); err != nil {
		return nil, err
	}
	_, _, derived, err := h.deps.Library.DeriveWorkload(ctx, workloadSpecFrom(req.StroppyVersion, req.Protocol, req.Segments, req.Options))
	if err != nil {
		if e, ok := errs.AsValidation(err); ok {
			if vr, ok := e.Validation.(*schemapb.ValidationResult); ok {
				return &oas.WorkloadPreview{Validation: validationOf(vr), SegmentsSummary: []oas.WorkloadPreviewSegmentsSummaryItem{}}, nil
			}
		}
		return nil, err
	}
	return &oas.WorkloadPreview{
		Validation:      oas.ValidationResult{Errors: []oas.ValidationError{}},
		Requirements:    oas.NewOptRequirements(oas.Requirements{"runner": requirementOf(derived.Runner)}),
		SegmentsSummary: segmentsSummaryOf(derived.Segments),
	}, nil
}

// CloneWorkload — copy.
func (h *Handler) CloneWorkload(ctx context.Context, req oas.OptCloneRequest, params oas.CloneWorkloadParams) (*oas.Workload, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	name := ""
	if r, ok := req.Get(); ok {
		name = r.Name.Or("")
	}
	w, derived, err := h.deps.Library.CloneWorkload(ctx, a, t.ID, params.ID, name)
	if err != nil {
		return nil, err
	}
	return h.workloadOf(w, derived, nil), nil
}

// ExportWorkload — portable document.
func (h *Handler) ExportWorkload(ctx context.Context, params oas.ExportWorkloadParams) (oas.ExportWorkloadRes, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	doc, err := h.deps.Library.ExportWorkload(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	if wantsYAML(ctx) {
		r, err := yamlDocument(doc)
		if err != nil {
			return nil, err
		}
		return &oas.ExportWorkloadOKApplicationYaml{Data: r}, nil
	}
	return exportDocumentOf(doc), nil
}

// ImportWorkload — create or update by name.
func (h *Handler) ImportWorkload(ctx context.Context, req oas.ImportWorkloadReq, params oas.ImportWorkloadParams) (*oas.Workload, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	doc, err := documentOf(req)
	if err != nil {
		return nil, err
	}
	w, derived, _, err := h.deps.Library.ImportWorkload(ctx, a, t.ID, doc)
	if err != nil {
		return nil, err
	}
	return h.workloadOf(w, derived, nil), nil
}

// DiffWorkloads — field-by-field diff.
func (h *Handler) DiffWorkloads(ctx context.Context, params oas.DiffWorkloadsParams) (*oas.Diff, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	changes, err := h.deps.Library.DiffWorkloads(ctx, a, t.ID, params.A, params.B)
	if err != nil {
		return nil, err
	}
	return diffOf(changes), nil
}
