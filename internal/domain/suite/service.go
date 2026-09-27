package suite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Pipeline is the Graphene pipeline id of a suite run.
const Pipeline = "stroppy-suite"

// Webhook event names.
const (
	EventSuiteStarted  = "suite.started"
	EventSuiteFinished = "suite.finished"
	// EventSuiteFailed shares the finished event with status=failed: the
	// webhook vocabulary has no separate suite.failed.
	EventSuiteFailed = "suite.finished"
)

// Service is the suite use cases.
type Service struct {
	repo      Repository
	runs      *run.Service
	runRepo   run.Repository
	graphene  run.Graphene
	access    Access
	library   *library.Service
	publisher run.Publisher
	audit     *audit.Service
	scope     func(ctx context.Context, namespace string) context.Context
}

// NewService wires the use cases.
func NewService(repo Repository, runs *run.Service, runRepo run.Repository, g run.Graphene, access Access, lib *library.Service, publisher run.Publisher, auditSvc *audit.Service, scope func(context.Context, string) context.Context) *Service {
	return &Service{repo: repo, runs: runs, runRepo: runRepo, graphene: g, access: access, library: lib, publisher: publisher, audit: auditSvc, scope: scope}
}

func (s *Service) member(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, _, err := s.access.RoleIn(ctx, actor, tenantID)
	return err
}

func (s *Service) writer(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, role, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return err
	}
	if role == string(tenant.RoleViewer) {
		return errs.Forbidden("viewers cannot change suites")
	}
	return nil
}

func (s *Service) owned(ctx context.Context, tenantID, id uuid.UUID) (Suite, error) {
	x, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Suite{}, err
	}
	if x.TenantID != tenantID {
		return Suite{}, errs.NotFound("suite")
	}
	return x, nil
}

func (s *Service) ownedRun(ctx context.Context, tenantID, id uuid.UUID) (SuiteRun, error) {
	r, err := s.repo.RunByID(ctx, id)
	if err != nil {
		return SuiteRun{}, err
	}
	if r.TenantID != tenantID || r.DeletedAt != nil {
		return SuiteRun{}, errs.NotFound("suite run")
	}
	r.Runs, err = s.runRepo.OfSuiteRun(ctx, id)
	return r, err
}

// --- definitions ------------------------------------------------------------

// normalize checks the tests and gives inline tests stable ids.
func (s *Service) normalize(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, tests []TestEntry) ([]TestEntry, error) {
	out := make([]TestEntry, 0, len(tests))
	for i, t := range tests {
		switch {
		case t.Ref != nil:
			if _, _, _, err := s.library.GetTest(ctx, actor, tenantID, *t.Ref); err != nil {
				return nil, errs.Invalid(fmt.Sprintf("tests[%d]: %v", i, err))
			}
		case t.Inline != nil:
			if t.InlineID == uuid.Nil {
				t.InlineID = uuid.New()
			}
			if t.InlineName == "" {
				t.InlineName = fmt.Sprintf("inline-%d", i+1)
			}
		default:
			return nil, errs.Invalid(fmt.Sprintf("tests[%d]: ref or inline", i))
		}
		out = append(out, t)
	}
	return out, nil
}

// Create stores a definition.
func (s *Service) Create(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, w Write) (Computed, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Computed{}, err
	}
	if strings.TrimSpace(w.Name) == "" {
		return Computed{}, errs.Invalid("name is required")
	}
	tests, err := s.normalize(ctx, actor, tenantID, w.Tests)
	if err != nil {
		return Computed{}, err
	}
	if _, taken, err := s.repo.ByName(ctx, tenantID, w.Name); err != nil {
		return Computed{}, err
	} else if taken {
		return Computed{}, errs.Conflict("a suite with this name exists")
	}
	now := time.Now().UTC()
	x := Suite{
		ID: uuid.New(), TenantID: tenantID, Name: w.Name, Description: w.Description, Tags: w.Tags, AuthorID: authorOf(actor),
		Tests: tests, Axes: w.Axes, Cells: w.Cells, Concurrency: max(1, w.Concurrency), Defaults: w.Defaults, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Insert(ctx, x); err != nil {
		return Computed{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "suite.create", Target: audit.Target{Kind: "suite", ID: x.ID.String(), Name: x.Name}}) //nolint:errcheck // audit never blocks
	return s.compute(ctx, actor, x)
}

// Get reads one suite with its cells.
func (s *Service) Get(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Computed, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Computed{}, err
	}
	x, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Computed{}, err
	}
	return s.compute(ctx, actor, x)
}

// List reads the tenant's suites (cells computed without validation).
func (s *Service) List(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q library.ListQuery, favorites bool) ([]Computed, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	fav := ""
	if favorites && actor.UserID != uuid.Nil {
		fav = actor.UserID.String()
	}
	list, err := s.repo.List(ctx, tenantID, q, fav)
	if err != nil {
		return nil, err
	}
	out := make([]Computed, 0, len(list))
	for _, x := range list {
		c := Computed{Suite: x, Cells: Generate(x, s.testNames(ctx, actor, x))}
		c.Summary = s.summary(ctx, x, c.Cells)
		out = append(out, c)
	}
	return out, nil
}

// Update applies a patch.
func (s *Service) Update(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, p Patch) (Computed, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Computed{}, err
	}
	if _, err := s.owned(ctx, tenantID, id); err != nil {
		return Computed{}, err
	}
	if p.SetTests {
		tests, err := s.normalize(ctx, actor, tenantID, p.Tests)
		if err != nil {
			return Computed{}, err
		}
		p.Tests = tests
	}
	if p.Name != nil {
		if other, taken, err := s.repo.ByName(ctx, tenantID, *p.Name); err != nil {
			return Computed{}, err
		} else if taken && other != id {
			return Computed{}, errs.Conflict("a suite with this name exists")
		}
	}
	if err := s.repo.Update(ctx, id, p); err != nil {
		return Computed{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "suite.update", Target: audit.Target{Kind: "suite", ID: id.String()}}) //nolint:errcheck // audit never blocks
	x, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Computed{}, err
	}
	return s.compute(ctx, actor, x)
}

// Delete removes a definition (runs keep their snapshot).
func (s *Service) Delete(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return err
	}
	if _, err := s.owned(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "suite.delete", Target: audit.Target{Kind: "suite", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return nil
}

// Clone copies a definition under a new name.
func (s *Service) Clone(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, name string) (Computed, error) {
	x, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Computed{}, err
	}
	if name == "" {
		name = x.Name + " (copy)"
	}
	return s.Create(ctx, actor, tenantID, Write{Name: name, Description: x.Description, Tags: x.Tags, Tests: x.Tests, Axes: x.Axes, Cells: x.Cells, Concurrency: x.Concurrency, Defaults: x.Defaults})
}

// Export renders the portable document.
func (s *Service) Export(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (library.Document, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return library.Document{}, err
	}
	x, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return library.Document{}, err
	}
	specRaw, err := json.Marshal(map[string]any{"tests": x.Tests, "axes": x.Axes, "cells": x.Cells, "concurrency": x.Concurrency, "defaults": x.Defaults})
	if err != nil {
		return library.Document{}, err
	}
	return library.Document{APIVersion: library.APIVersion, Kind: "Suite", Metadata: library.Metadata{Name: x.Name, Description: x.Description, Tags: x.Tags}, Spec: specRaw}, nil
}

// Import creates or updates by name.
func (s *Service) Import(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, doc library.Document) (Computed, bool, error) {
	if doc.Kind != "Suite" {
		return Computed{}, false, errs.Invalid("document kind must be Suite")
	}
	var sp struct {
		Tests       []TestEntry `json:"tests"`
		Axes        Axes        `json:"axes"`
		Cells       []Cell      `json:"cells"`
		Concurrency int         `json:"concurrency"`
		Defaults    Defaults    `json:"defaults"`
	}
	if err := json.Unmarshal(doc.Spec, &sp); err != nil {
		return Computed{}, false, errs.Wrap(errs.CodeInvalid, "spec", err)
	}
	w := Write{Name: doc.Metadata.Name, Description: doc.Metadata.Description, Tags: doc.Metadata.Tags, Tests: sp.Tests, Axes: sp.Axes, Cells: sp.Cells, Concurrency: sp.Concurrency, Defaults: sp.Defaults}
	if id, taken, err := s.repo.ByName(ctx, tenantID, w.Name); err != nil {
		return Computed{}, false, err
	} else if taken {
		desc, axes, def := w.Description, w.Axes, w.Defaults
		c, err := s.Update(ctx, actor, tenantID, id, Patch{Description: &desc, Tags: w.Tags, Tests: w.Tests, SetTests: true, Axes: &axes, Cells: w.Cells, SetCells: true, Concurrency: &w.Concurrency, Defaults: &def})
		return c, false, err
	}
	c, err := s.Create(ctx, actor, tenantID, w)
	return c, true, err
}

// ImportDocument implements the examples' suite port.
func (s *Service) ImportDocument(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, doc library.Document) ([]library.Usage, error) {
	c, _, err := s.Import(ctx, actor, tenantID, doc)
	if err != nil {
		return nil, err
	}
	return []library.Usage{{Kind: "suite", ID: c.Suite.ID, Name: c.Suite.Name}}, nil
}

// Preview computes the cells of an unsaved definition with validation.
func (s *Service) Preview(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, w Write) (Preview, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Preview{}, err
	}
	tests, err := s.normalize(ctx, actor, tenantID, w.Tests)
	if err != nil {
		return Preview{}, err
	}
	x := Suite{TenantID: tenantID, Tests: tests, Axes: w.Axes, Cells: w.Cells, Concurrency: max(1, w.Concurrency), Defaults: w.Defaults}
	c, err := s.compute(ctx, actor, x)
	if err != nil {
		return Preview{}, err
	}
	out := Preview{Cells: c.Cells, Fits: c.Fits, Fit: library.Fit{Fits: true, Issues: []library.Issue{}}}
	byProfile := map[uuid.UUID]*QuotaCheck{}
	for _, cell := range c.Cells {
		if !cell.Enabled {
			continue
		}
		fit := c.Fits[cell.ID]
		if !fit.Fits {
			out.Fit.Fits = false
			for _, i := range fit.Issues {
				i.Path = "cells." + cell.ID + "." + i.Path
				out.Fit.Issues = append(out.Fit.Issues, i)
			}
		}
		res := c.Resolved[cell.ID]
		for _, m := range res.Machines {
			out.Totals.Machines += m.Count
			out.Totals.CPU += m.CPU * m.Count
			out.Totals.MemoryGB += m.MemoryGB * m.Count
			out.Totals.DiskGB += m.DiskGB * m.Count
		}
		if res.Profile != nil {
			q := byProfile[res.Profile.ID]
			if q == nil {
				q = &QuotaCheck{ProviderProfileID: res.Profile.ID, Fits: true, Issues: []string{}}
				byProfile[res.Profile.ID] = q
			}
			if res.Profile.Status != "ready" {
				q.Fits = false
				q.Issues = append(q.Issues, fmt.Sprintf("profile is %s", res.Profile.Status))
			}
		}
	}
	for _, q := range byProfile {
		out.QuotaCheck = append(out.QuotaCheck, *q)
	}
	sort.Slice(out.QuotaCheck, func(i, j int) bool {
		return out.QuotaCheck[i].ProviderProfileID.String() < out.QuotaCheck[j].ProviderProfileID.String()
	})
	return out, nil
}

// compute generates the cells and validates each enabled one.
func (s *Service) compute(ctx context.Context, actor auth.Actor, x Suite) (Computed, error) {
	names := s.testNames(ctx, actor, x)
	c := Computed{Suite: x, Cells: Generate(x, names), Fits: map[string]library.Fit{}, Resolved: map[string]library.Resolved{}}
	for _, cell := range c.Cells {
		if !cell.Enabled {
			continue
		}
		testSpec, _, err := s.cellSpec(ctx, actor, x, cell)
		if err != nil {
			c.Fits[cell.ID] = library.Fit{Issues: []library.Issue{{Path: "axis", Code: "invalid", Severity: "ERROR", Message: err.Error()}}}
			continue
		}
		fit, res, err := s.library.Validate(ctx, x.TenantID, testSpec)
		if err != nil {
			return Computed{}, err
		}
		c.Fits[cell.ID], c.Resolved[cell.ID] = fit, res
	}
	if x.ID != uuid.Nil {
		c.Summary = s.summary(ctx, x, c.Cells)
	}
	return c, nil
}

func (s *Service) summary(ctx context.Context, x Suite, cells []Cell) Summary {
	sum := Summary{CellCount: len(cells), Schedules: []run.Ref{}}
	for _, c := range cells {
		if c.Enabled {
			sum.EnabledCellCount++
		}
	}
	sum.RunCount, _ = s.repo.RunCountOfSuite(ctx, x.ID) //nolint:errcheck // header only
	if last, err := s.repo.RunsOfSuite(ctx, x.ID, 1, 0); err == nil && len(last) > 0 {
		sum.LastRun = &last[0]
	}
	if sch, err := s.repo.SchedulesOf(ctx, "suite", x.ID); err == nil {
		sum.Schedules = sch
	}
	return sum
}

// testNames resolves the display names of the referenced tests.
func (s *Service) testNames(ctx context.Context, actor auth.Actor, x Suite) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	for _, t := range x.Tests {
		if t.Ref != nil {
			if test, _, _, err := s.library.GetTest(ctx, actor, x.TenantID, *t.Ref); err == nil {
				out[*t.Ref] = test.Name
			}
		} else {
			out[t.InlineID] = t.InlineName
		}
	}
	return out
}

// cellSpec resolves the test of a cell and applies its axis + defaults.
func (s *Service) cellSpec(ctx context.Context, actor auth.Actor, x Suite, cell Cell) (library.TestSpec, run.Overrides, error) {
	var base library.TestSpec
	found := false
	for _, t := range x.Tests {
		if t.ID() != cell.TestID {
			continue
		}
		found = true
		if t.Ref != nil {
			test, _, _, err := s.library.GetTest(ctx, actor, x.TenantID, *t.Ref)
			if err != nil {
				return base, run.Overrides{}, err
			}
			base = test.Spec
		} else {
			base = *t.Inline
		}
	}
	if !found {
		return base, run.Overrides{}, fmt.Errorf("cell %s: test %s is not in the suite", cell.ID, cell.TestID)
	}
	// Defaults under the cell's own overrides.
	if cell.Overrides.Keep == nil && x.Defaults.Keep != nil {
		cell.Overrides.Keep = x.Defaults.Keep
	}
	if cell.Overrides.RatingTenant == nil && x.Defaults.RatingTenant != nil {
		cell.Overrides.RatingTenant = x.Defaults.RatingTenant
	}
	if cell.Overrides.RatingGlobal == nil && x.Defaults.RatingGlobal != nil {
		cell.Overrides.RatingGlobal = x.Defaults.RatingGlobal
	}
	var db *library.DatabaseSpec
	var wl *library.WorkloadSpec
	if cell.Axis.DatabaseVersion != "" {
		switch {
		case base.DatabaseInline != nil:
			db = base.DatabaseInline
		case base.DatabaseRef != nil:
			d, _, _, err := s.library.GetDatabase(ctx, actor, x.TenantID, *base.DatabaseRef)
			if err != nil {
				return base, run.Overrides{}, err
			}
			db = &d.Spec
		}
	}
	if cell.Axis.WorkloadVariant != "" {
		switch {
		case base.WorkloadInline != nil:
			wl = base.WorkloadInline
		case base.WorkloadRef != nil:
			w, _, _, err := s.library.GetWorkload(ctx, actor, x.TenantID, *base.WorkloadRef)
			if err != nil {
				return base, run.Overrides{}, err
			}
			wl = &w.Spec
		}
	}
	return applyAxis(base, cell, x.Axes.WorkloadVariants, db, wl)
}

// UsingTest lists the suites referencing a test.
func (s *Service) UsingTest(ctx context.Context, testID uuid.UUID) ([]library.Usage, error) {
	return s.repo.UsingTest(ctx, testID)
}

// --- runs -------------------------------------------------------------------

// LaunchSuite prepares every enabled cell, stores the suite run and its
// child runs, then starts the stroppy-suite pipeline.
//
//nolint:funlen // one linear checklist
func (s *Service) LaunchSuite(ctx context.Context, actor auth.Actor, tenantID, suiteID uuid.UUID, in Launch, idempotencyKey string) (SuiteRun, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return SuiteRun{}, err
	}
	var sr SuiteRun
	var prepared []run.Prepared
	var params spec.Suite
	replayed := false
	err := s.runRepo.Admit(ctx, tenantID, func(ctx context.Context) error {
		if idempotencyKey != "" {
			id, ok, err := s.repo.RunByIdempotencyKey(ctx, tenantID, idempotencyKey)
			if err != nil {
				return err
			}
			if ok {
				sr, err = s.ownedRun(ctx, tenantID, id)
				replayed = true
				return err
			}
		}
		var err error
		sr, prepared, params, err = s.prepareLaunch(ctx, actor, tenantID, suiteID, in, idempotencyKey)
		return err
	})
	if err != nil {
		return SuiteRun{}, err
	}
	if replayed {
		return sr, nil
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "suite.launch", Target: audit.Target{Kind: "suite_run", ID: sr.ID.String(), Name: sr.Name}, Details: map[string]any{"suite": suiteID, "cells": len(sr.Cells)}}) //nolint:errcheck // audit never blocks
	labels := map[string]string{"stroppy.io/tenant": params.Tenant, "stroppy.io/suite-run": sr.ID.String()}
	if err := run.Submit(s.scope(ctx, sr.GrapheneNamespace), s.graphene, sr.CreatedAt, sr.ID.String(), Pipeline, params, labels); err != nil {
		if !errors.Is(err, run.ErrStartRejected) {
			// All child specs were committed with the suite; recovery does
			// not consult mutable library records or launch children itself.
			return s.ownedRun(ctx, tenantID, sr.ID)
		}
		reason := "start: " + err.Error()
		fin := time.Now().UTC()
		_ = s.repo.SetRunStatus(ctx, sr.ID, run.StatusFailed, reason, nil, &fin) //nolint:errcheck // reported below
		for _, p := range prepared {
			_ = s.runRepo.SetStatus(ctx, p.Run.ID, run.StatusFailed, run.PhaseDone, reason, nil, &fin) //nolint:errcheck // best-effort
		}
		return SuiteRun{}, errs.Wrap(errs.CodeUnavailable, "graphene did not accept the suite run", err)
	}
	for _, p := range prepared {
		s.runs.MarkStarted(ctx, p.Run)
	}
	s.publish(ctx, tenantID, EventSuiteStarted, sr)
	return s.ownedRun(ctx, tenantID, sr.ID)
}

// prepareLaunch validates and persists the whole matrix atomically under tenant admission.
func (s *Service) prepareLaunch(ctx context.Context, actor auth.Actor, tenantID, suiteID uuid.UUID, in Launch, idempotencyKey string) (SuiteRun, []run.Prepared, spec.Suite, error) {
	x, err := s.owned(ctx, tenantID, suiteID)
	if err != nil {
		return SuiteRun{}, nil, spec.Suite{}, err
	}
	cells := Generate(x, s.testNames(ctx, actor, x))
	wanted := map[string]bool{}
	for _, id := range in.CellIDs {
		wanted[id] = true
	}
	var chosen []Cell
	for _, c := range cells {
		if len(wanted) > 0 && !wanted[c.ID] {
			continue
		}
		if len(wanted) == 0 && !c.Enabled {
			continue
		}
		chosen = append(chosen, c)
	}
	if len(chosen) == 0 {
		return SuiteRun{}, nil, spec.Suite{}, errs.Invalid("no cells to run")
	}
	concurrency := x.Concurrency
	if in.Concurrency > 0 {
		concurrency = in.Concurrency
	}
	slug, ns, err := s.tenantScope(ctx, actor, tenantID)
	if err != nil {
		return SuiteRun{}, nil, spec.Suite{}, err
	}
	trigger := in.Trigger
	if trigger == "" {
		trigger = run.TriggerSuite
	}
	name := in.Name
	if name == "" {
		name = x.Name
	}
	srID := uuid.New()
	prepared := make([]run.Prepared, 0, len(chosen))
	cellRuns := make([]CellRun, 0, len(chosen))
	specCells := make([]spec.SuiteCell, 0, len(chosen))
	for i, c := range chosen {
		testSpec, o, err := s.cellSpec(ctx, actor, x, c)
		if err != nil {
			return SuiteRun{}, nil, spec.Suite{}, errs.Invalid(err.Error())
		}
		if in.Keep != nil {
			o.Keep = in.Keep
		}
		if in.RatingTenant != nil {
			o.RatingTenant = in.RatingTenant
		}
		if in.RatingGlobal != nil {
			o.RatingGlobal = in.RatingGlobal
		}
		o.Name = fmt.Sprintf("%s · %s", name, orDefault(c.Name, c.ID))
		o.Trigger, o.SuiteRunID, o.CellID, o.ScheduleID = run.TriggerSuite, &srID, c.ID, in.ScheduleID
		o.Labels = mergeLabels(in.Labels, o.Labels)
		var testRef *run.Ref
		if c.TestID != uuid.Nil {
			testRef = &run.Ref{ID: c.TestID, Name: c.TestName}
		}
		if testRef != nil && isInline(x, c.TestID) {
			testRef = nil
		}
		p, err := s.runs.Prepare(ctx, actor, tenantID, testSpec, testRef, o, run.PrepareOptions{GrapheneRunID: srID.String() + "-" + c.ID, ExtraLive: min(i, concurrency-1)})
		if err != nil {
			if e, ok := errors.AsType[*errs.Error](err); ok {
				e.Detail = "cell " + c.ID + ": " + e.Detail
			}
			return SuiteRun{}, nil, spec.Suite{}, err
		}
		var rs spec.Run
		if err := json.Unmarshal(p.Compiled.Spec, &rs); err != nil {
			return SuiteRun{}, nil, spec.Suite{}, errs.Wrap(errs.CodeInternal, "run spec", err)
		}
		prepared = append(prepared, p)
		cellRuns = append(cellRuns, CellRun{CellID: c.ID, Name: c.Name, RunID: p.Run.ID})
		specCells = append(specCells, spec.SuiteCell{ID: c.ID, RunSpec: rs})
	}
	now := time.Now().UTC()
	sr := SuiteRun{
		ID: srID, TenantID: tenantID, SuiteID: &x.ID, SuiteName: x.Name, Name: name, Status: run.StatusPending, Trigger: trigger,
		ScheduleID: in.ScheduleID, RetryOf: in.RetryOf, Concurrency: concurrency, Cells: cellRuns, Labels: in.Labels, AuthorID: authorOf(actor),
		GrapheneNamespace: ns, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.InsertRun(ctx, sr, idempotencyKey); err != nil {
		return SuiteRun{}, nil, spec.Suite{}, err
	}
	for _, p := range prepared {
		if err := s.runs.Insert(ctx, p.Run); err != nil {
			return SuiteRun{}, nil, spec.Suite{}, err
		}
	}
	params := spec.Suite{SuiteRunID: srID.String(), Tenant: slug, Cells: specCells, Concurrency: concurrency, Defaults: spec.SuiteDefaults{ContinueOnFailure: true, Labels: in.Labels}}
	return sr, prepared, params, nil
}

func isInline(x Suite, id uuid.UUID) bool {
	for _, t := range x.Tests {
		if t.Inline != nil && t.InlineID == id {
			return true
		}
	}
	return false
}

func mergeLabels(a, b map[string]string) map[string]string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func (s *Service) tenantScope(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, ns string, err error) {
	slug, _, err = s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return "", "", err
	}
	ns, err = s.access.NamespaceOf(ctx, tenantID)
	return slug, ns, err
}

func (s *Service) publish(ctx context.Context, tenantID uuid.UUID, event string, r SuiteRun) {
	if s.publisher == nil {
		return
	}
	p := r.Progress()
	_ = s.publisher.Publish(ctx, tenantID, event, map[string]any{"id": r.ID, "name": r.Name, "status": r.Status, "suite_id": r.SuiteID, "progress": map[string]int{"total": p.Total, "done": p.Done, "failed": p.Failed}}) //nolint:errcheck // best-effort
}

// Peek reads a suite run without an access check (the caller resolves
// access from its tenant).
func (s *Service) Peek(ctx context.Context, id uuid.UUID) (SuiteRun, error) {
	r, err := s.repo.RunByID(ctx, id)
	if err != nil {
		return SuiteRun{}, err
	}
	if r.DeletedAt != nil {
		return SuiteRun{}, errs.NotFound("suite run")
	}
	r.Runs, err = s.runRepo.OfSuiteRun(ctx, id)
	return r, err
}

// GetRun reads a suite run with its child runs.
func (s *Service) GetRun(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (SuiteRun, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return SuiteRun{}, err
	}
	return s.ownedRun(ctx, tenantID, id)
}

// Runs lists suite runs.
func (s *Service) Runs(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q ListQuery) ([]SuiteRun, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	list, err := s.repo.Runs(ctx, tenantID, q)
	if err != nil {
		return nil, err
	}
	return s.withRuns(ctx, list)
}

// RunsOfSuite lists the runs of one suite.
func (s *Service) RunsOfSuite(ctx context.Context, actor auth.Actor, tenantID, suiteID uuid.UUID, limit, offset int) ([]SuiteRun, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	if _, err := s.owned(ctx, tenantID, suiteID); err != nil {
		return nil, err
	}
	list, err := s.repo.RunsOfSuite(ctx, suiteID, limit, offset)
	if err != nil {
		return nil, err
	}
	return s.withRuns(ctx, list)
}

func (s *Service) withRuns(ctx context.Context, list []SuiteRun) ([]SuiteRun, error) {
	for i := range list {
		runs, err := s.runRepo.OfSuiteRun(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i].Runs = runs
	}
	return list, nil
}

// CancelRun stops the suite and its live cells.
func (s *Service) CancelRun(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (SuiteRun, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return SuiteRun{}, err
	}
	r, err := s.ownedRun(ctx, tenantID, id)
	if err != nil {
		return SuiteRun{}, err
	}
	if r.Status.Terminal() {
		return SuiteRun{}, errs.Conflict("suite run already finished")
	}
	// Mark first (suite and cells): the projectors may record terminal
	// statuses the moment Graphene acknowledges; terminal never moves back.
	if err := s.repo.SetRunStatus(ctx, id, run.StatusCancelling, "cancel requested", nil, nil); err != nil {
		return SuiteRun{}, err
	}
	for _, child := range r.Runs {
		if !child.Status.Terminal() {
			_ = s.runRepo.SetStatus(ctx, child.ID, run.StatusCancelling, child.Phase, "suite cancelled", nil, nil) //nolint:errcheck // best-effort
		}
	}
	if err := s.graphene.CancelRun(s.scope(ctx, r.GrapheneNamespace), r.ID.String()); err != nil {
		return SuiteRun{}, errs.Wrap(errs.CodeUnavailable, "cancel", err)
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "suite_run.cancel", Target: audit.Target{Kind: "suite_run", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return s.ownedRun(ctx, tenantID, id)
}

// DeleteRun soft-deletes the suite run and its cells, cascading Graphene.
func (s *Service) DeleteRun(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return err
	}
	r, err := s.ownedRun(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !r.Status.Terminal() {
		return errs.Conflict("cancel the suite run before deleting it")
	}
	if err := s.graphene.DeleteRef(s.scope(ctx, r.GrapheneNamespace), r.GrapheneRef()); err != nil {
		return errs.Wrap(errs.CodeUnavailable, "delete graphene resources", err)
	}
	for _, child := range r.Runs {
		_ = s.runRepo.SoftDelete(ctx, child.ID) //nolint:errcheck // best-effort
	}
	if err := s.repo.DeleteRun(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "suite_run.delete", Target: audit.Target{Kind: "suite_run", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return nil
}

// RetryFailed launches a new suite run over the cells that failed or were
// cancelled.
func (s *Service) RetryFailed(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (SuiteRun, error) {
	r, err := s.GetRun(ctx, actor, tenantID, id)
	if err != nil {
		return SuiteRun{}, err
	}
	if r.SuiteID == nil {
		return SuiteRun{}, errs.Conflict("the suite of this run is gone")
	}
	var cells []string
	for _, child := range r.Runs {
		if child.Status == run.StatusFailed || child.Status == run.StatusCancelled {
			cells = append(cells, child.CellID)
		}
	}
	if len(cells) == 0 {
		return SuiteRun{}, errs.Conflict("no failed cells")
	}
	return s.LaunchSuite(ctx, actor, tenantID, *r.SuiteID, Launch{Name: r.Name + " (retry)", Concurrency: r.Concurrency, CellIDs: cells, Labels: r.Labels, RetryOf: &r.ID}, "")
}

// SummaryRow is one cell of the results table.
type SummaryRow struct {
	CellID  string
	Name    string
	Run     *run.Run
	Metrics map[string]MetricCell
}

// MetricCell is one value with its comparison verdict.
type MetricCell struct {
	Value    float64
	Baseline *float64
	DiffPct  *float64
	Verdict  string // better|worse|same|missing
}

// RunSummary is the results table with an optional comparison to another
// suite run (default: the previous run of the same suite).
type RunSummary struct {
	SuiteRunID uuid.UUID
	ComparedTo *uuid.UUID
	MetricKeys []string
	Rows       []SummaryRow
}

// Summary builds the results table.
func (s *Service) Summary(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, compareTo *uuid.UUID) (RunSummary, error) {
	r, err := s.GetRun(ctx, actor, tenantID, id)
	if err != nil {
		return RunSummary{}, err
	}
	var base *SuiteRun
	switch {
	case compareTo != nil:
		b, err := s.ownedRun(ctx, tenantID, *compareTo)
		if err != nil {
			return RunSummary{}, err
		}
		base = &b
	case r.SuiteID != nil:
		prev, err := s.repo.RunsOfSuite(ctx, *r.SuiteID, 20, 0)
		if err == nil {
			for i := range prev {
				if prev[i].ID != r.ID && prev[i].CreatedAt.Before(r.CreatedAt) && prev[i].Status == run.StatusCompleted {
					b := prev[i]
					b.Runs, _ = s.runRepo.OfSuiteRun(ctx, b.ID) //nolint:errcheck // comparison is best-effort
					base = &b
					break
				}
			}
		}
	}
	baseByCell := map[string]run.Run{}
	if base != nil {
		for _, x := range base.Runs {
			baseByCell[x.CellID] = x
		}
	}
	keys := map[string]bool{}
	out := RunSummary{SuiteRunID: r.ID}
	if base != nil {
		out.ComparedTo = &base.ID
	}
	childByCell := map[string]run.Run{}
	for _, x := range r.Runs {
		childByCell[x.CellID] = x
	}
	for _, c := range r.Cells {
		row := SummaryRow{CellID: c.CellID, Name: c.Name, Metrics: map[string]MetricCell{}}
		child, ok := childByCell[c.CellID]
		if ok {
			cc := child
			row.Run = &cc
			for k, v := range child.Summary.Headline {
				keys[k] = true
				m := MetricCell{Value: v, Verdict: "missing"}
				if b, ok := baseByCell[c.CellID]; ok {
					if bv, ok := b.Summary.Headline[k]; ok {
						bvv := bv
						m.Baseline = &bvv
						m.Verdict = verdict(k, v, bv)
						if bv != 0 {
							d := (v - bv) / bv * 100
							m.DiffPct = &d
						}
					}
				}
				row.Metrics[k] = m
			}
		}
		out.Rows = append(out.Rows, row)
	}
	for k := range keys {
		out.MetricKeys = append(out.MetricKeys, k)
	}
	sort.Strings(out.MetricKeys)
	return out, nil
}

// verdict compares a metric against its baseline; latency and errors are
// lower-is-better, everything else higher.
func verdict(key string, v, base float64) string {
	const tolerance = 0.02
	if base == 0 {
		if v == 0 {
			return "same"
		}
		return "missing"
	}
	diff := (v - base) / base
	if diff > -tolerance && diff < tolerance {
		return "same"
	}
	lowerBetter := strings.Contains(key, "latency") || strings.Contains(key, "error") || strings.Contains(key, "p9") || strings.Contains(key, "p50")
	if (diff > 0) != lowerBetter {
		return "better"
	}
	return "worse"
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func authorOf(a auth.Actor) *uuid.UUID {
	if a.UserID == uuid.Nil {
		return nil
	}
	id := a.UserID
	return &id
}
