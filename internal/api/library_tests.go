package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-faster/jx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func sizesFrom(in oas.OptRoleSizes) map[string]library.RoleSize {
	v, ok := in.Get()
	if !ok {
		return nil
	}
	out := map[string]library.RoleSize{}
	for role, item := range v {
		rs := library.RoleSize{Size: string(item.Size)}
		if v, ok := item.Machine.Get(); ok {
			rs.Machine = rawOf(v)
		}
		if d, ok := item.Disk.Get(); ok {
			rs.DiskType = d.Type.Or("")
			rs.DiskGB = d.GB.Or(0)
		}
		out[role] = rs
	}
	return out
}

func sizesTo(in map[string]library.RoleSize) oas.RoleSizes {
	out := oas.RoleSizes{}
	for role, rs := range in {
		item := oas.RoleSizesItem{Size: oas.Size(rs.Size)}
		if len(rs.Machine) > 0 {
			item.Machine = oas.NewOptSchemaValue(schemaValueOf(rs.Machine))
		}
		if rs.DiskType != "" || rs.DiskGB > 0 {
			d := oas.RoleSizesItemDisk{}
			if rs.DiskType != "" {
				d.Type = oas.NewOptString(rs.DiskType)
			}
			if rs.DiskGB > 0 {
				d.GB = oas.NewOptInt(rs.DiskGB)
			}
			item.Disk = oas.NewOptRoleSizesItemDisk(d)
		}
		out[role] = item
	}
	return out
}

func keepFrom(s oas.OptString) (*time.Duration, error) {
	v, ok := s.Get()
	if !ok {
		return nil, nil //nolint:nilnil // absent = keep
	}
	if v == "" {
		d := time.Duration(0)
		return &d, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return nil, errs.Invalid("keep is not a duration")
	}
	return &d, nil
}

// testSpecFrom builds the spec of a create/validate body.
func testSpecFrom(req *oas.TestWrite) (library.TestSpec, error) {
	spec := library.TestSpec{Sizes: sizesFrom(req.Sizes), RatingTenant: true}
	if v, ok := req.Execution.Get(); ok {
		spec.Execution = rawOf(v)
	}
	if db, ok := req.Database.Get(); ok {
		switch db.Type {
		case oas.TestWriteDatabase0TestWriteDatabase:
			id := db.TestWriteDatabase0.Ref.ID
			spec.DatabaseRef = &id
		case oas.TestWriteDatabase1TestWriteDatabase:
			inline := databaseSpecOf(db.TestWriteDatabase1.Inline)
			spec.DatabaseInline = &inline
		}
	}
	if wl, ok := req.Workload.Get(); ok {
		switch wl.Type {
		case oas.TestWriteWorkload0TestWriteWorkload:
			id := wl.TestWriteWorkload0.Ref.ID
			spec.WorkloadRef = &id
		case oas.TestWriteWorkload1TestWriteWorkload:
			inline := workloadSpecOf(wl.TestWriteWorkload1.Inline)
			spec.WorkloadInline = &inline
		}
	}
	if v, ok := req.ProviderProfileID.Get(); ok {
		spec.ProviderProfileID = &v
	}
	keep, err := keepFrom(req.Keep)
	if err != nil {
		return library.TestSpec{}, err
	}
	if keep != nil {
		spec.Keep = *keep
	}
	if r, ok := req.Rating.Get(); ok {
		spec.RatingTenant = r.Tenant.Or(true)
		spec.RatingGlobal = r.Global.Or(false)
	}
	return spec, nil
}

func (h *Handler) fitOf(fit library.Fit) oas.Fit {
	out := oas.Fit{Fits: fit.Fits, Issues: make([]oas.FitIssuesItem, 0, len(fit.Issues))}
	for _, i := range fit.Issues {
		item := oas.FitIssuesItem{Path: i.Path, Code: i.Code, Severity: oas.FitIssuesItemSeverity(i.Severity)}
		item.Scope = oas.NewOptValidationScope(validationScope(i.Scope))
		if i.Message != "" {
			item.Message = oas.NewOptString(i.Message)
		}
		if i.Suggested != nil {
			if raw, err := json.Marshal(i.Suggested); err == nil {
				item.Suggested = jx.Raw(raw)
			}
		}
		out.Issues = append(out.Issues, item)
	}
	if fit.StaleDatabase || fit.StaleWorkload {
		out.Stale = oas.NewOptFitStale(oas.FitStale{Database: oas.NewOptBool(fit.StaleDatabase), Workload: oas.NewOptBool(fit.StaleWorkload)})
	}
	return out
}

func (h *Handler) resolvedOf(res library.Resolved) oas.TestResolved {
	out := oas.TestResolved{}
	if res.Database != nil {
		derived := library.DatabaseDerived{}
		if res.DatabaseDerived != nil {
			derived = *res.DatabaseDerived
		}
		out.Database = oas.NewOptDatabase(*h.databaseOf(*res.Database, derived, nil))
	}
	if res.Workload != nil {
		derived := library.WorkloadDerived{}
		if res.WorkloadDerived != nil {
			derived = *res.WorkloadDerived
		}
		out.Workload = oas.NewOptWorkload(*h.workloadOf(*res.Workload, derived, nil))
	}
	return out
}

func estimatedOf(res library.Resolved) oas.TestValidationEstimated {
	out := oas.TestValidationEstimated{Machines: make([]oas.TestValidationEstimatedMachinesItem, 0, len(res.Machines))}
	for _, m := range res.Machines {
		out.Machines = append(out.Machines, oas.TestValidationEstimatedMachinesItem{
			Role: oas.NewOptString(m.Role), Count: oas.NewOptInt(m.Count), Size: oas.NewOptSize(oas.Size(m.Size)),
			CPU: oas.NewOptInt(m.CPU), MemoryGB: oas.NewOptInt(m.MemoryGB), DiskGB: oas.NewOptInt(m.DiskGB),
		})
	}
	return out
}

func (h *Handler) testOf(t library.Test, fit library.Fit, res library.Resolved) *oas.Test {
	hd := entityHeader(t.Entity)
	out := &oas.Test{
		ID: hd.id, Name: hd.name, Description: hd.description, Tags: oas.NewOptTestTags(tagsOf(t.Tags)), Author: hd.author, CreatedAt: hd.created, UpdatedAt: hd.updated,
		Status: oas.TestStatus(t.Status), Sizes: oas.NewOptRoleSizes(sizesTo(t.Spec.Sizes)), Usages: []oas.Usage{},
		Rating:     oas.NewOptRatingFlags(oas.RatingFlags{Tenant: oas.NewOptBool(t.Spec.RatingTenant), Global: oas.NewOptBool(t.Spec.RatingGlobal)}),
		Validation: oas.NewOptFit(h.fitOf(fit)), Requirements: oas.NewOptRequirements(requirementsOf(res.Requirements)),
		Resolved: oas.NewOptTestResolved(h.resolvedOf(res)),
	}
	if len(t.Spec.Execution) > 0 {
		out.Execution = oas.NewOptSchemaValue(schemaValueOf(t.Spec.Execution))
	}
	switch {
	case t.Spec.DatabaseRef != nil:
		ref := oas.Ref{ID: *t.Spec.DatabaseRef}
		if res.Database != nil {
			ref.Name = oas.NewOptString(res.Database.Name)
		}
		out.Database = oas.NewOptTestDatabase(oas.TestDatabase{Type: oas.TestDatabase0TestDatabase, TestDatabase0: oas.TestDatabase0{Ref: ref}})
	case t.Spec.DatabaseInline != nil:
		out.Database = oas.NewOptTestDatabase(oas.TestDatabase{Type: oas.TestDatabase1TestDatabase, TestDatabase1: oas.TestDatabase1{Inline: specToWire(*t.Spec.DatabaseInline)}})
	}
	switch {
	case t.Spec.WorkloadRef != nil:
		ref := oas.Ref{ID: *t.Spec.WorkloadRef}
		if res.Workload != nil {
			ref.Name = oas.NewOptString(res.Workload.Name)
		}
		out.Workload = oas.NewOptTestWorkload(oas.TestWorkload{Type: oas.TestWorkload0TestWorkload, TestWorkload0: oas.TestWorkload0{Ref: ref}})
	case t.Spec.WorkloadInline != nil:
		out.Workload = oas.NewOptTestWorkload(oas.TestWorkload{Type: oas.TestWorkload1TestWorkload, TestWorkload1: oas.TestWorkload1{Inline: workloadSpecToWire(*t.Spec.WorkloadInline)}})
	}
	if t.Spec.ProviderProfileID != nil {
		out.ProviderProfileID = oas.NewOptNilUUID(*t.Spec.ProviderProfileID)
	}
	if t.Spec.Keep > 0 {
		out.Keep = oas.NewOptString(t.Spec.Keep.String())
	}
	summary := oas.TestSummary{}
	if res.DatabaseDerived != nil {
		summary.TopologyLabel = oas.NewOptString(res.DatabaseDerived.Plan.Label)
		summary.NodeCount = oas.NewOptInt(res.DatabaseDerived.Plan.NodeCount())
	}
	if res.Database != nil {
		summary.DbKind = oas.NewOptDatabaseKind(oas.DatabaseKind(res.Database.Spec.Kind))
		summary.DbVersion = oas.NewOptString(res.Database.Spec.Version)
	} else if t.Spec.DatabaseInline != nil {
		summary.DbKind = oas.NewOptDatabaseKind(oas.DatabaseKind(t.Spec.DatabaseInline.Kind))
		summary.DbVersion = oas.NewOptString(t.Spec.DatabaseInline.Version)
	}
	if res.Workload != nil {
		summary.Protocol = oas.NewOptProtocol(oas.Protocol(res.Workload.Spec.Protocol))
		summary.StroppyVersion = oas.NewOptString(res.Workload.Spec.StroppyVersion)
	} else if t.Spec.WorkloadInline != nil {
		summary.Protocol = oas.NewOptProtocol(oas.Protocol(t.Spec.WorkloadInline.Protocol))
		summary.StroppyVersion = oas.NewOptString(t.Spec.WorkloadInline.StroppyVersion)
	}
	out.Summary = oas.NewOptTestSummary(summary)
	return out
}

// ListTests — tests of the tenant.
func (h *Handler) ListTests(ctx context.Context, params oas.ListTestsParams) (*oas.ListTestsOK, error) {
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
	for _, s := range params.Status {
		q.Statuses = append(q.Statuses, string(s))
	}
	list, err := h.deps.Library.ListTests(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, q.Offset, limit)
	out := &oas.ListTestsOK{Data: make([]oas.Test, 0, len(list)), Meta: meta}
	for _, item := range list {
		fit, res, verr := h.deps.Library.Validate(ctx, t.ID, item.Spec)
		if verr != nil {
			return nil, verr
		}
		out.Data = append(out.Data, *h.testOf(item, fit, res))
	}
	return out, nil
}

// CreateTest — store (draft until it fits).
func (h *Handler) CreateTest(ctx context.Context, req *oas.TestWrite, params oas.CreateTestParams) (*oas.Test, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	spec, err := testSpecFrom(req)
	if err != nil {
		return nil, err
	}
	w := library.EntityWrite{Name: req.Name, Description: req.Description.Or("")}
	if tags, ok := req.Tags.Get(); ok {
		w.Tags = tags
	}
	test, fit, res, err := h.deps.Library.CreateTest(ctx, a, t.ID, w, spec)
	if err != nil {
		return nil, err
	}
	return h.testOf(test, fit, res), nil
}

// GetTest — with resolved references and validation.
func (h *Handler) GetTest(ctx context.Context, params oas.GetTestParams) (*oas.Test, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	test, fit, res, err := h.deps.Library.GetTest(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.testOf(test, fit, res), nil
}

// PatchTest — incremental update, status recomputed.
func (h *Handler) PatchTest(ctx context.Context, req *oas.TestPatch, params oas.PatchTestParams) (*oas.Test, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p := library.TestPatch{Sizes: sizesFrom(req.Sizes)}
	if v, ok := req.Execution.Get(); ok {
		p.Execution = rawOf(v)
	}
	if v, ok := req.Name.Get(); ok {
		p.Name = &v
	}
	if v, ok := req.Description.Get(); ok {
		p.Description = &v
	}
	if v, ok := req.Tags.Get(); ok {
		p.Tags = v
	}
	if db, ok := req.Database.Get(); ok {
		p.SetDatabase = true
		switch db.Type {
		case oas.TestPatchDatabase0TestPatchDatabase:
			id := db.TestPatchDatabase0.Ref.ID
			p.DatabaseRef = &id
		case oas.TestPatchDatabase1TestPatchDatabase:
			inline := databaseSpecOf(db.TestPatchDatabase1.Inline)
			p.DatabaseInline = &inline
		}
	}
	if wl, ok := req.Workload.Get(); ok {
		p.SetWorkload = true
		switch wl.Type {
		case oas.TestPatchWorkload0TestPatchWorkload:
			id := wl.TestPatchWorkload0.Ref.ID
			p.WorkloadRef = &id
		case oas.TestPatchWorkload1TestPatchWorkload:
			inline := workloadSpecOf(wl.TestPatchWorkload1.Inline)
			p.WorkloadInline = &inline
		}
	}
	if req.ProviderProfileID.Set {
		p.SetProvider = true
		if !req.ProviderProfileID.Null {
			id := req.ProviderProfileID.Value
			p.ProviderProfileID = &id
		}
	}
	keep, err := keepFrom(req.Keep)
	if err != nil {
		return nil, err
	}
	p.Keep = keep
	if r, ok := req.Rating.Get(); ok {
		if v, ok := r.Tenant.Get(); ok {
			p.RatingTenant = &v
		}
		if v, ok := r.Global.Get(); ok {
			p.RatingGlobal = &v
		}
	}
	test, fit, res, err := h.deps.Library.UpdateTest(ctx, a, t.ID, params.ID, p)
	if err != nil {
		return nil, err
	}
	return h.testOf(test, fit, res), nil
}

// DeleteTest — remove.
func (h *Handler) DeleteTest(ctx context.Context, params oas.DeleteTestParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Library.DeleteTest(ctx, a, t.ID, params.ID)
}

// ValidateTest — live form validation.
func (h *Handler) ValidateTest(ctx context.Context, req *oas.TestWrite, params oas.ValidateTestParams) (*oas.TestValidation, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	spec, err := testSpecFrom(req)
	if err != nil {
		return nil, err
	}
	fit, res, err := h.deps.Library.ValidateSpec(ctx, a, t.ID, spec)
	if err != nil {
		return nil, err
	}
	status := oas.TestStatusDraft
	if fit.Fits {
		status = oas.TestStatusReady
	}
	resolved := h.resolvedOf(res)
	return &oas.TestValidation{
		Status: status, Validation: h.fitOf(fit), Requirements: oas.NewOptRequirements(requirementsOf(res.Requirements)),
		Resolved:  oas.NewOptTestValidationResolved(oas.TestValidationResolved(resolved)),
		Estimated: oas.NewOptTestValidationEstimated(estimatedOf(res)),
	}, nil
}

// CloneTest — copy.
func (h *Handler) CloneTest(ctx context.Context, req oas.OptCloneRequest, params oas.CloneTestParams) (*oas.Test, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	name := ""
	if r, ok := req.Get(); ok {
		name = r.Name.Or("")
	}
	test, fit, res, err := h.deps.Library.CloneTest(ctx, a, t.ID, params.ID, name)
	if err != nil {
		return nil, err
	}
	return h.testOf(test, fit, res), nil
}

// ExportTest — portable document.
func (h *Handler) ExportTest(ctx context.Context, params oas.ExportTestParams) (oas.ExportTestRes, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	doc, err := h.deps.Library.ExportTest(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	if wantsYAML(ctx) {
		r, err := yamlDocument(doc)
		if err != nil {
			return nil, err
		}
		return &oas.ExportTestOKApplicationYaml{Data: r}, nil
	}
	return exportDocumentOf(doc), nil
}

// ImportTest — create or update by name.
func (h *Handler) ImportTest(ctx context.Context, req oas.ImportTestReq, params oas.ImportTestParams) (*oas.Test, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	doc, err := documentOf(req)
	if err != nil {
		return nil, err
	}
	test, fit, res, _, err := h.deps.Library.ImportTest(ctx, a, t.ID, doc)
	if err != nil {
		return nil, err
	}
	return h.testOf(test, fit, res), nil
}

// DiffTests — field-by-field diff.
func (h *Handler) DiffTests(ctx context.Context, params oas.DiffTestsParams) (*oas.Diff, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	changes, err := h.deps.Library.DiffTests(ctx, a, t.ID, params.A, params.B)
	if err != nil {
		return nil, err
	}
	return diffOf(changes), nil
}
