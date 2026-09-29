package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-faster/jx"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/suite"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// --- wire mapping -----------------------------------------------------------

func suiteOverridesFrom(o oas.OptLaunchOverrides) (suite.Overrides, error) {
	out := suite.Overrides{}
	v, ok := o.Get()
	if !ok {
		return out, nil
	}
	out.Name = v.Name.Or("")
	if id, ok := v.ProviderProfileID.Get(); ok {
		out.ProviderProfileID = &id
	}
	out.Sizes = sizesFrom(v.Sizes)
	keep, err := keepFrom(v.Keep)
	if err != nil {
		return out, err
	}
	out.Keep = keep
	if r, ok := v.Rating.Get(); ok {
		if t, ok := r.Tenant.Get(); ok {
			out.RatingTenant = &t
		}
		if g, ok := r.Global.Get(); ok {
			out.RatingGlobal = &g
		}
	}
	if l, ok := v.Labels.Get(); ok {
		out.Labels = l
	}
	out.Notes = v.Notes.Or("")
	return out, nil
}

func suiteOverridesTo(o suite.Overrides) oas.OptLaunchOverrides {
	if o.Name == "" && o.ProviderProfileID == nil && len(o.Sizes) == 0 && o.Keep == nil && o.RatingTenant == nil && o.RatingGlobal == nil && len(o.Labels) == 0 && o.Notes == "" {
		return oas.OptLaunchOverrides{}
	}
	out := oas.LaunchOverrides{}
	if o.Name != "" {
		out.Name = oas.NewOptString(o.Name)
	}
	if o.ProviderProfileID != nil {
		out.ProviderProfileID = oas.NewOptUUID(*o.ProviderProfileID)
	}
	if len(o.Sizes) > 0 {
		out.Sizes = oas.NewOptRoleSizes(sizesTo(o.Sizes))
	}
	if o.Keep != nil {
		out.Keep = oas.NewOptString(o.Keep.String())
	}
	if o.RatingTenant != nil || o.RatingGlobal != nil {
		r := oas.RatingFlags{}
		if o.RatingTenant != nil {
			r.Tenant = oas.NewOptBool(*o.RatingTenant)
		}
		if o.RatingGlobal != nil {
			r.Global = oas.NewOptBool(*o.RatingGlobal)
		}
		out.Rating = oas.NewOptRatingFlags(r)
	}
	if len(o.Labels) > 0 {
		out.Labels = oas.NewOptLaunchOverridesLabels(oas.LaunchOverridesLabels(o.Labels))
	}
	if o.Notes != "" {
		out.Notes = oas.NewOptString(o.Notes)
	}
	return oas.NewOptLaunchOverrides(out)
}

func axesFrom(a oas.OptSuiteAxes) suite.Axes {
	v, ok := a.Get()
	if !ok {
		return suite.Axes{}
	}
	out := suite.Axes{ProviderProfiles: v.ProviderProfiles, DatabaseVersions: v.DatabaseVersions}
	for _, s := range v.Sizes {
		out.Sizes = append(out.Sizes, sizesFrom(oas.NewOptRoleSizes(s)))
	}
	for i, w := range v.WorkloadVariants {
		wv := suite.WorkloadVariant{Name: w.Name.Or(""), VUs: w.Vus.Or(0), ScaleFactor: w.ScaleFactor.Or(0), Duration: w.Duration.Or("")}
		if wv.Name == "" {
			wv.Name = "variant-" + itoa(i+1)
		}
		if p, ok := w.Params.Get(); ok {
			wv.Params = map[string]any{}
			for k, raw := range p {
				var x any
				if json.Unmarshal(raw, &x) == nil {
					wv.Params[k] = x
				}
			}
		}
		out.WorkloadVariants = append(out.WorkloadVariants, wv)
	}
	return out
}

func axesTo(a suite.Axes) oas.OptSuiteAxes {
	if len(a.ProviderProfiles) == 0 && len(a.Sizes) == 0 && len(a.DatabaseVersions) == 0 && len(a.WorkloadVariants) == 0 {
		return oas.OptSuiteAxes{}
	}
	out := oas.SuiteAxes{ProviderProfiles: a.ProviderProfiles, DatabaseVersions: a.DatabaseVersions, Sizes: []oas.RoleSizes{}, WorkloadVariants: []oas.SuiteAxesWorkloadVariantsItem{}}
	if out.ProviderProfiles == nil {
		out.ProviderProfiles = []uuid.UUID{}
	}
	if out.DatabaseVersions == nil {
		out.DatabaseVersions = []string{}
	}
	for _, s := range a.Sizes {
		out.Sizes = append(out.Sizes, sizesTo(s))
	}
	for _, w := range a.WorkloadVariants {
		item := oas.SuiteAxesWorkloadVariantsItem{Name: oas.NewOptString(w.Name)}
		if w.VUs > 0 {
			item.Vus = oas.NewOptInt(w.VUs)
		}
		if w.ScaleFactor > 0 {
			item.ScaleFactor = oas.NewOptInt(w.ScaleFactor)
		}
		if w.Duration != "" {
			item.Duration = oas.NewOptString(w.Duration)
		}
		if len(w.Params) > 0 {
			p := oas.SuiteAxesWorkloadVariantsItemParams{}
			for k, v := range w.Params {
				if raw, err := json.Marshal(v); err == nil {
					p[k] = jx.Raw(raw)
				}
			}
			item.Params = oas.NewOptSuiteAxesWorkloadVariantsItemParams(p)
		}
		out.WorkloadVariants = append(out.WorkloadVariants, item)
	}
	return oas.NewOptSuiteAxes(out)
}

func cellsFrom(in []oas.SuiteCell) ([]suite.Cell, error) {
	out := make([]suite.Cell, 0, len(in))
	for _, c := range in {
		o, err := suiteOverridesFrom(c.Overrides)
		if err != nil {
			return nil, err
		}
		cell := suite.Cell{ID: c.ID, Name: c.Name.Or(""), TestID: c.Test.ID, TestName: c.Test.Name.Or(""), Enabled: c.Enabled, Overrides: o, Generated: c.Generated.Or(false)}
		if ax, ok := c.Axis.Get(); ok {
			if id, ok := ax.ProviderProfileID.Get(); ok {
				cell.Axis.ProviderProfileID = &id
			}
			cell.Axis.Sizes = sizesFrom(ax.Sizes)
			cell.Axis.DatabaseVersion = ax.DatabaseVersion.Or("")
			cell.Axis.WorkloadVariant = ax.WorkloadVariant.Or("")
		}
		out = append(out, cell)
	}
	return out, nil
}

func (h *Handler) cellTo(c suite.Cell, fit *library.Fit) oas.SuiteCell {
	out := oas.SuiteCell{ID: c.ID, Test: oas.Ref{ID: c.TestID}, Enabled: c.Enabled, Generated: oas.NewOptBool(c.Generated), Overrides: suiteOverridesTo(c.Overrides)}
	if c.Name != "" {
		out.Name = oas.NewOptString(c.Name)
	}
	if c.TestName != "" {
		out.Test.Name = oas.NewOptString(c.TestName)
	}
	ax := oas.SuiteCellAxis{}
	set := false
	if c.Axis.ProviderProfileID != nil {
		ax.ProviderProfileID, set = oas.NewOptUUID(*c.Axis.ProviderProfileID), true
	}
	if len(c.Axis.Sizes) > 0 {
		ax.Sizes, set = oas.NewOptRoleSizes(sizesTo(c.Axis.Sizes)), true
	}
	if c.Axis.DatabaseVersion != "" {
		ax.DatabaseVersion, set = oas.NewOptString(c.Axis.DatabaseVersion), true
	}
	if c.Axis.WorkloadVariant != "" {
		ax.WorkloadVariant, set = oas.NewOptString(c.Axis.WorkloadVariant), true
	}
	if set {
		out.Axis = oas.NewOptSuiteCellAxis(ax)
	}
	if fit != nil {
		out.Validation = oas.NewOptFit(h.fitOf(*fit))
	}
	return out
}

func testsFromWrite(in []oas.SuiteWriteTestsItem) ([]suite.TestEntry, error) {
	out := make([]suite.TestEntry, 0, len(in))
	for _, t := range in {
		switch t.Type {
		case oas.SuiteWriteTestsItem0SuiteWriteTestsItem:
			id := t.SuiteWriteTestsItem0.Ref.ID
			out = append(out, suite.TestEntry{Ref: &id})
		case oas.SuiteWriteTestsItem1SuiteWriteTestsItem:
			spec, err := testSpecFrom(&t.SuiteWriteTestsItem1.Inline)
			if err != nil {
				return nil, err
			}
			out = append(out, suite.TestEntry{Inline: &spec, InlineName: t.SuiteWriteTestsItem1.Inline.Name})
		}
	}
	return out, nil
}

func testsFromPatch(in []oas.SuitePatchTestsItem) ([]suite.TestEntry, error) {
	out := make([]suite.TestEntry, 0, len(in))
	for _, t := range in {
		switch t.Type {
		case oas.SuitePatchTestsItem0SuitePatchTestsItem:
			id := t.SuitePatchTestsItem0.Ref.ID
			out = append(out, suite.TestEntry{Ref: &id})
		case oas.SuitePatchTestsItem1SuitePatchTestsItem:
			spec, err := testSpecFrom(&t.SuitePatchTestsItem1.Inline)
			if err != nil {
				return nil, err
			}
			out = append(out, suite.TestEntry{Inline: &spec, InlineName: t.SuitePatchTestsItem1.Inline.Name})
		}
	}
	return out, nil
}

func testsTo(in []suite.TestEntry, names map[uuid.UUID]string) []oas.SuiteTestsItem {
	out := make([]oas.SuiteTestsItem, 0, len(in))
	for _, t := range in {
		if t.Ref != nil {
			ref := oas.Ref{ID: *t.Ref}
			if n := names[*t.Ref]; n != "" {
				ref.Name = oas.NewOptString(n)
			}
			out = append(out, oas.SuiteTestsItem{Type: oas.SuiteTestsItem0SuiteTestsItem, SuiteTestsItem0: oas.SuiteTestsItem0{Ref: ref}})
			continue
		}
		out = append(out, oas.SuiteTestsItem{Type: oas.SuiteTestsItem1SuiteTestsItem, SuiteTestsItem1: oas.SuiteTestsItem1{Inline: testWriteOf(t.InlineName, *t.Inline)}})
	}
	return out
}

// testWriteOf renders an inline test spec back as a TestWrite.
func testWriteOf(name string, s library.TestSpec) oas.TestWrite {
	out := oas.TestWrite{Name: name, Sizes: oas.NewOptRoleSizes(sizesTo(s.Sizes)), Rating: oas.NewOptRatingFlags(oas.RatingFlags{Tenant: oas.NewOptBool(s.RatingTenant), Global: oas.NewOptBool(s.RatingGlobal)})}
	switch {
	case s.DatabaseRef != nil:
		out.Database = oas.NewOptTestWriteDatabase(oas.TestWriteDatabase{Type: oas.TestWriteDatabase0TestWriteDatabase, TestWriteDatabase0: oas.TestWriteDatabase0{Ref: oas.Ref{ID: *s.DatabaseRef}}})
	case s.DatabaseInline != nil:
		out.Database = oas.NewOptTestWriteDatabase(oas.TestWriteDatabase{Type: oas.TestWriteDatabase1TestWriteDatabase, TestWriteDatabase1: oas.TestWriteDatabase1{Inline: specToWire(*s.DatabaseInline)}})
	}
	switch {
	case s.WorkloadRef != nil:
		out.Workload = oas.NewOptTestWriteWorkload(oas.TestWriteWorkload{Type: oas.TestWriteWorkload0TestWriteWorkload, TestWriteWorkload0: oas.TestWriteWorkload0{Ref: oas.Ref{ID: *s.WorkloadRef}}})
	case s.WorkloadInline != nil:
		out.Workload = oas.NewOptTestWriteWorkload(oas.TestWriteWorkload{Type: oas.TestWriteWorkload1TestWriteWorkload, TestWriteWorkload1: oas.TestWriteWorkload1{Inline: workloadSpecToWire(*s.WorkloadInline)}})
	}
	if s.ProviderProfileID != nil {
		out.ProviderProfileID = oas.NewOptNilUUID(*s.ProviderProfileID)
	}
	if s.Keep > 0 {
		out.Keep = oas.NewOptString(s.Keep.String())
	}
	return out
}

func defaultsFrom(rating oas.OptRatingFlags, keep oas.OptString) (suite.Defaults, error) {
	out := suite.Defaults{}
	if r, ok := rating.Get(); ok {
		if t, ok := r.Tenant.Get(); ok {
			out.RatingTenant = &t
		}
		if g, ok := r.Global.Get(); ok {
			out.RatingGlobal = &g
		}
	}
	k, err := keepFrom(keep)
	if err != nil {
		return out, err
	}
	out.Keep = k
	return out, nil
}

func defaultsTo(d suite.Defaults) oas.OptSuiteDefaults {
	if d.RatingTenant == nil && d.RatingGlobal == nil && d.Keep == nil {
		return oas.OptSuiteDefaults{}
	}
	out := oas.SuiteDefaults{}
	if d.RatingTenant != nil || d.RatingGlobal != nil {
		r := oas.RatingFlags{}
		if d.RatingTenant != nil {
			r.Tenant = oas.NewOptBool(*d.RatingTenant)
		}
		if d.RatingGlobal != nil {
			r.Global = oas.NewOptBool(*d.RatingGlobal)
		}
		out.Rating = oas.NewOptRatingFlags(r)
	}
	if d.Keep != nil {
		out.Keep = oas.NewOptString(d.Keep.String())
	}
	return oas.NewOptSuiteDefaults(out)
}

func (h *Handler) suiteOf(c suite.Computed, favorite *bool) *oas.Suite {
	s := c.Suite
	names := map[uuid.UUID]string{}
	for _, cell := range c.Cells {
		names[cell.TestID] = cell.TestName
	}
	out := &oas.Suite{
		ID: s.ID, Name: s.Name, Tags: oas.NewOptSuiteTags(oas.SuiteTags(tagsOf(s.Tags))), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, DeletedAt: oas.OptNilDateTime{Null: true, Set: true},
		Tests: testsTo(s.Tests, names), Axes: axesTo(s.Axes), Cells: make([]oas.SuiteCell, 0, len(s.Cells)), Concurrency: oas.NewOptInt(s.Concurrency), Defaults: defaultsTo(s.Defaults),
		ComputedCells: make([]oas.SuiteCell, 0, len(c.Cells)),
	}
	if s.Description != "" {
		out.Description = oas.NewOptString(s.Description)
	}
	if s.AuthorID != nil {
		out.Author.ID = s.AuthorID.String()
	}
	if favorite != nil {
		out.IsFavorite = oas.NewOptBool(*favorite)
	}
	for _, cell := range s.Cells {
		out.Cells = append(out.Cells, h.cellTo(cell, nil))
	}
	for _, cell := range c.Cells {
		var fit *library.Fit
		if f, ok := c.Fits[cell.ID]; ok {
			ff := f
			fit = &ff
		}
		out.ComputedCells = append(out.ComputedCells, h.cellTo(cell, fit))
	}
	sum := oas.SuiteSummary{CellCount: oas.NewOptInt(c.Summary.CellCount), EnabledCellCount: oas.NewOptInt(c.Summary.EnabledCellCount), RunCount: oas.NewOptInt(c.Summary.RunCount), Schedules: []oas.Ref{}}
	for _, r := range c.Summary.Schedules {
		sum.Schedules = append(sum.Schedules, oas.Ref{ID: r.ID, Name: oas.NewOptString(r.Name)})
	}
	if c.Summary.LastRun != nil {
		sum.LastRun = oas.NewOptRunRef(oas.RunRef{ID: c.Summary.LastRun.ID, Name: oas.NewOptString(c.Summary.LastRun.Name), Status: oas.NewOptRunStatus(oas.RunStatus(c.Summary.LastRun.Status)), StartedAt: optTime(c.Summary.LastRun.StartedAt)})
	}
	out.Summary = oas.NewOptSuiteSummary(sum)
	return out
}

func (h *Handler) suiteRunOf(r suite.SuiteRun) *oas.SuiteRun {
	p := r.Progress()
	out := &oas.SuiteRun{
		ID: r.ID, Name: r.Name, Suite: oas.Ref{Name: oas.NewOptString(r.SuiteName)}, Status: oas.RunStatus(r.Status), Trigger: oas.Trigger(r.Trigger),
		Concurrency: oas.NewOptInt(r.Concurrency), Labels: oas.NewOptSuiteRunLabels(oas.SuiteRunLabels(tagsOf(r.Labels))), CreatedAt: r.CreatedAt, StartedAt: optTime(r.StartedAt), FinishedAt: optTime(r.FinishedAt),
		Progress: oas.SuiteRunProgress{Total: p.Total, Done: p.Done, Failed: p.Failed, Running: p.Running, Pending: p.Pending, Cancelled: oas.NewOptInt(p.Cancelled), Pct: oas.NewOptFloat64(p.Pct())},
		Cells:    make([]oas.SuiteRunCellsItem, 0, len(r.Cells)), Graphene: oas.NewOptSuiteRunGraphene(oas.SuiteRunGraphene{RunRef: oas.NewOptString(r.GrapheneRef())}),
	}
	if r.SuiteID != nil {
		out.Suite.ID = *r.SuiteID
	}
	if r.ScheduleID != nil || r.RetryOf != nil {
		out.TriggerRef = oas.NewOptSuiteRunTriggerRef(oas.SuiteRunTriggerRef{ScheduleID: optUUID(r.ScheduleID), RetryOf: optUUID(r.RetryOf)})
	}
	if r.AuthorID != nil {
		out.Author.ID = r.AuthorID.String()
	}
	if r.DurationSeconds != nil {
		out.Duration = oas.NewOptNilString((time.Duration(*r.DurationSeconds * float64(time.Second))).Round(time.Second).String())
	} else {
		out.Duration = oas.OptNilString{Null: true, Set: true}
	}
	byCell := map[string]run.Run{}
	for _, x := range r.Runs {
		byCell[x.CellID] = x
	}
	for _, c := range r.Cells {
		item := oas.SuiteRunCellsItem{CellID: c.CellID, Status: oas.RunStatusPending}
		if c.Name != "" {
			item.Name = oas.NewOptString(c.Name)
		}
		if x, ok := byCell[c.CellID]; ok {
			item.Status = oas.RunStatus(x.Status)
			item.Run = oas.NewOptRunRef(oas.RunRef{ID: x.ID, Name: oas.NewOptString(x.Name), Status: oas.NewOptRunStatus(oas.RunStatus(x.Status)), StartedAt: optTime(x.StartedAt)})
			item.Summary = oas.NewOptRunSummary(runSummaryOf(x))
		}
		out.Cells = append(out.Cells, item)
	}
	return out
}

func itoa(i int) string { return fmtInt(i) }

// --- suites -----------------------------------------------------------------

func suiteWriteFrom(req *oas.SuiteWrite) (suite.Write, error) {
	tests, err := testsFromWrite(req.Tests)
	if err != nil {
		return suite.Write{}, err
	}
	cells, err := cellsFrom(req.Cells)
	if err != nil {
		return suite.Write{}, err
	}
	w := suite.Write{Name: req.Name, Description: req.Description.Or(""), Tests: tests, Axes: axesFrom(req.Axes), Cells: cells, Concurrency: req.Concurrency.Or(1)}
	if t, ok := req.Tags.Get(); ok {
		w.Tags = t
	}
	if d, ok := req.Defaults.Get(); ok {
		if w.Defaults, err = defaultsFrom(d.Rating, d.Keep); err != nil {
			return suite.Write{}, err
		}
	}
	return w, nil
}

// ListSuites — definitions of the tenant.
func (h *Handler) ListSuites(ctx context.Context, params oas.ListSuitesParams) (*oas.ListSuitesOK, error) {
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
	list, err := h.deps.Suites.List(ctx, a, t.ID, q, params.Favorites.Or(false))
	if err != nil {
		return nil, err
	}
	list, meta := page(list, q.Offset, limit)
	favs, _ := h.deps.Runs.Favorites(ctx, a, t.ID, "suite") //nolint:errcheck // flag only
	out := &oas.ListSuitesOK{Data: make([]oas.Suite, 0, len(list)), Meta: meta}
	for _, c := range list {
		f := favs[c.Suite.ID]
		out.Data = append(out.Data, *h.suiteOf(c, &f))
	}
	return out, nil
}

// CreateSuite — store a definition.
func (h *Handler) CreateSuite(ctx context.Context, req *oas.SuiteWrite, params oas.CreateSuiteParams) (*oas.Suite, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	w, err := suiteWriteFrom(req)
	if err != nil {
		return nil, err
	}
	c, err := h.deps.Suites.Create(ctx, a, t.ID, w)
	if err != nil {
		return nil, err
	}
	return h.suiteOf(c, nil), nil
}

// GetSuite — one definition with computed cells.
func (h *Handler) GetSuite(ctx context.Context, params oas.GetSuiteParams) (*oas.Suite, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	c, err := h.deps.Suites.Get(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	favs, _ := h.deps.Runs.Favorites(ctx, a, t.ID, "suite") //nolint:errcheck // flag only
	f := favs[c.Suite.ID]
	return h.suiteOf(c, &f), nil
}

// PatchSuite — partial update.
func (h *Handler) PatchSuite(ctx context.Context, req *oas.SuitePatch, params oas.PatchSuiteParams) (*oas.Suite, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p := suite.Patch{}
	if v, ok := req.Name.Get(); ok {
		p.Name = &v
	}
	if v, ok := req.Description.Get(); ok {
		p.Description = &v
	}
	if v, ok := req.Tags.Get(); ok {
		p.Tags = v
	}
	if req.Tests != nil {
		tests, err := testsFromPatch(req.Tests)
		if err != nil {
			return nil, err
		}
		p.Tests, p.SetTests = tests, true
	}
	if v, ok := req.Axes.Get(); ok {
		axes := axesFrom(oas.NewOptSuiteAxes(v))
		p.Axes = &axes
	}
	if req.Cells != nil {
		cells, err := cellsFrom(req.Cells)
		if err != nil {
			return nil, err
		}
		p.Cells, p.SetCells = cells, true
	}
	if v, ok := req.Concurrency.Get(); ok {
		p.Concurrency = &v
	}
	if d, ok := req.Defaults.Get(); ok {
		def, err := defaultsFrom(d.Rating, d.Keep)
		if err != nil {
			return nil, err
		}
		p.Defaults = &def
	}
	c, err := h.deps.Suites.Update(ctx, a, t.ID, params.ID, p)
	if err != nil {
		return nil, err
	}
	return h.suiteOf(c, nil), nil
}

// DeleteSuite — remove.
func (h *Handler) DeleteSuite(ctx context.Context, params oas.DeleteSuiteParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Suites.Delete(ctx, a, t.ID, params.ID)
}

// PreviewSuite — cells + validation of an unsaved definition.
func (h *Handler) PreviewSuite(ctx context.Context, req *oas.SuiteWrite, params oas.PreviewSuiteParams) (*oas.SuitePreview, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	w, err := suiteWriteFrom(req)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Suites.Preview(ctx, a, t.ID, w)
	if err != nil {
		return nil, err
	}
	out := &oas.SuitePreview{Cells: make([]oas.SuiteCell, 0, len(p.Cells)), Validation: h.fitOf(p.Fit), QuotaCheck: []oas.SuitePreviewQuotaCheckItem{}}
	for _, c := range p.Cells {
		var fit *library.Fit
		if f, ok := p.Fits[c.ID]; ok {
			ff := f
			fit = &ff
		}
		out.Cells = append(out.Cells, h.cellTo(c, fit))
	}
	out.Totals = oas.NewOptSuitePreviewTotals(oas.SuitePreviewTotals{Machines: oas.NewOptInt(p.Totals.Machines), CPU: oas.NewOptInt(p.Totals.CPU), MemoryGB: oas.NewOptInt(p.Totals.MemoryGB), DiskGB: oas.NewOptInt(p.Totals.DiskGB)})
	for _, q := range p.QuotaCheck {
		out.QuotaCheck = append(out.QuotaCheck, oas.SuitePreviewQuotaCheckItem{ProviderProfileID: oas.NewOptUUID(q.ProviderProfileID), Fits: oas.NewOptBool(q.Fits), Issues: q.Issues})
	}
	return out, nil
}

// CloneSuite — copy.
func (h *Handler) CloneSuite(ctx context.Context, req oas.OptCloneRequest, params oas.CloneSuiteParams) (*oas.Suite, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	name := ""
	if r, ok := req.Get(); ok {
		name = r.Name.Or("")
	}
	c, err := h.deps.Suites.Clone(ctx, a, t.ID, params.ID, name)
	if err != nil {
		return nil, err
	}
	return h.suiteOf(c, nil), nil
}

// ExportSuite — portable document.
func (h *Handler) ExportSuite(ctx context.Context, params oas.ExportSuiteParams) (oas.ExportSuiteRes, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	doc, err := h.deps.Suites.Export(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	if wantsYAML(ctx) {
		r, err := yamlDocument(doc)
		if err != nil {
			return nil, err
		}
		return &oas.ExportSuiteOKApplicationYaml{Data: r}, nil
	}
	return exportDocumentOf(doc), nil
}

// ImportSuite — create or update by name.
func (h *Handler) ImportSuite(ctx context.Context, req oas.ImportSuiteReq, params oas.ImportSuiteParams) (*oas.Suite, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	doc, err := documentOf(req)
	if err != nil {
		return nil, err
	}
	c, _, err := h.deps.Suites.Import(ctx, a, t.ID, doc)
	if err != nil {
		return nil, err
	}
	return h.suiteOf(c, nil), nil
}

// --- suite runs -------------------------------------------------------------

// LaunchSuite — start a suite run.
func (h *Handler) LaunchSuite(ctx context.Context, req oas.OptSuiteLaunch, params oas.LaunchSuiteParams) (*oas.SuiteRun, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	in := suite.Launch{}
	if v, ok := req.Get(); ok {
		in.Name, in.Concurrency, in.CellIDs = v.Name.Or(""), v.Concurrency.Or(0), v.CellIds
		if r, ok := v.Rating.Get(); ok {
			if x, ok := r.Tenant.Get(); ok {
				in.RatingTenant = &x
			}
			if x, ok := r.Global.Get(); ok {
				in.RatingGlobal = &x
			}
		}
		if in.Keep, err = keepFrom(v.Keep); err != nil {
			return nil, err
		}
		if l, ok := v.Labels.Get(); ok {
			in.Labels = l
		}
	}
	r, err := h.deps.Suites.LaunchSuite(ctx, a, t.ID, params.ID, in, params.IdempotencyKey.Or(""))
	if err != nil {
		return nil, err
	}
	return h.suiteRunOf(r), nil
}

// ListSuiteRuns — filtered list.
func (h *Handler) ListSuiteRuns(ctx context.Context, params oas.ListSuiteRunsParams) (*oas.ListSuiteRunsOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	offset, err := cursorOffset(params.Cursor)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := suite.ListQuery{Sort: string(params.Sort.Or(oas.ListSuiteRunsSortStartedAt)), Desc: params.Order.Or(oas.OrderDesc) == oas.OrderDesc, Limit: limit + 1, Offset: offset}
	for _, s := range params.Status {
		q.Statuses = append(q.Statuses, string(s))
	}
	for _, tr := range params.Trigger {
		q.Triggers = append(q.Triggers, string(tr))
	}
	if v, ok := params.SuiteID.Get(); ok {
		q.SuiteID = v.String()
	}
	if v, ok := params.StartedAfter.Get(); ok {
		q.StartedAfter = v
	}
	if v, ok := params.StartedBefore.Get(); ok {
		q.StartedBefore = v
	}
	list, err := h.deps.Suites.Runs(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.ListSuiteRunsOK{Data: make([]oas.SuiteRun, 0, len(list)), Meta: meta}
	for _, r := range list {
		out.Data = append(out.Data, *h.suiteRunOf(r))
	}
	return out, nil
}

// ListSuiteRunsOfSuite — history of one suite.
func (h *Handler) ListSuiteRunsOfSuite(ctx context.Context, params oas.ListSuiteRunsOfSuiteParams) (*oas.ListSuiteRunsOfSuiteOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	offset, err := cursorOffset(params.Cursor)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	list, err := h.deps.Suites.RunsOfSuite(ctx, a, t.ID, params.ID, limit+1, offset)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.ListSuiteRunsOfSuiteOK{Data: make([]oas.SuiteRun, 0, len(list)), Meta: meta}
	for _, r := range list {
		out.Data = append(out.Data, *h.suiteRunOf(r))
	}
	return out, nil
}

// GetSuiteRun — one suite run with its cells.
func (h *Handler) GetSuiteRun(ctx context.Context, params oas.GetSuiteRunParams) (*oas.SuiteRun, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Suites.GetRun(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.suiteRunOf(r), nil
}

// GetSuiteRunSummary — results table with comparison.
func (h *Handler) GetSuiteRunSummary(ctx context.Context, params oas.GetSuiteRunSummaryParams) (*oas.SuiteRunSummary, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	var compareTo *uuid.UUID
	if v, ok := params.CompareTo.Get(); ok {
		compareTo = &v
	}
	s, err := h.deps.Suites.Summary(ctx, a, t.ID, params.ID, compareTo)
	if err != nil {
		return nil, err
	}
	out := &oas.SuiteRunSummary{SuiteRunID: s.SuiteRunID, MetricKeys: s.MetricKeys, Rows: make([]oas.SuiteRunSummaryRowsItem, 0, len(s.Rows)), ComparedTo: oas.OptNilUUID{Null: true, Set: true}}
	if out.MetricKeys == nil {
		out.MetricKeys = []string{}
	}
	if s.ComparedTo != nil {
		out.ComparedTo = oas.NewOptNilUUID(*s.ComparedTo)
	}
	for _, row := range s.Rows {
		item := oas.SuiteRunSummaryRowsItem{CellID: row.CellID}
		if row.Name != "" {
			item.Name = oas.NewOptString(row.Name)
		}
		if row.Run != nil {
			item.Run = oas.NewOptRunRef(oas.RunRef{ID: row.Run.ID, Name: oas.NewOptString(row.Run.Name), Status: oas.NewOptRunStatus(oas.RunStatus(row.Run.Status)), StartedAt: optTime(row.Run.StartedAt)})
			item.Status = oas.NewOptRunStatus(oas.RunStatus(row.Run.Status))
		}
		if len(row.Metrics) > 0 {
			m := oas.SuiteRunSummaryRowsItemMetrics{}
			for k, v := range row.Metrics {
				cell := oas.SuiteRunSummaryRowsItemMetricsItem{Value: oas.NewOptFloat64(v.Value), Verdict: oas.NewOptSuiteRunSummaryRowsItemMetricsItemVerdict(oas.SuiteRunSummaryRowsItemMetricsItemVerdict(v.Verdict))}
				if v.Baseline != nil {
					cell.Previous = oas.NewOptFloat64(*v.Baseline)
				}
				if v.DiffPct != nil {
					cell.DiffPct = oas.NewOptFloat64(*v.DiffPct)
				}
				m[k] = cell
			}
			item.Metrics = oas.NewOptSuiteRunSummaryRowsItemMetrics(m)
		}
		out.Rows = append(out.Rows, item)
	}
	return out, nil
}

// CancelSuiteRun — cascade cancel.
func (h *Handler) CancelSuiteRun(ctx context.Context, params oas.CancelSuiteRunParams) (*oas.SuiteRun, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Suites.CancelRun(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.suiteRunOf(r), nil
}

// DeleteSuiteRun — soft delete + Graphene cascade.
func (h *Handler) DeleteSuiteRun(ctx context.Context, params oas.DeleteSuiteRunParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Suites.DeleteRun(ctx, a, t.ID, params.ID)
}

// RetryFailedSuiteRun — new suite run over the failed cells.
func (h *Handler) RetryFailedSuiteRun(ctx context.Context, params oas.RetryFailedSuiteRunParams) (*oas.SuiteRun, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Suites.RetryFailed(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.suiteRunOf(r), nil
}

var _ = errs.CodeInvalid
