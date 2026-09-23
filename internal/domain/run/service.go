package run

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
)

// Service is the run use cases: launch (from a test, a previous run or a
// resumed stand), the lifecycle actions, reads and favorites. The
// projection of Graphene events into the stored state lives in Projector.
type Service struct {
	repo      Repository
	graphene  Graphene
	access    Access
	library   *library.Service
	profiles  Profiles
	limits    Limits
	compiler  Compiler
	publisher Publisher
	audit     *audit.Service
	// scope binds a Graphene call to a tenant namespace.
	scope func(ctx context.Context, namespace string) context.Context
}

// NewService wires the use cases.
func NewService(repo Repository, g Graphene, access Access, lib *library.Service, profiles Profiles, limits Limits, compiler Compiler, publisher Publisher, auditSvc *audit.Service, scope func(context.Context, string) context.Context) *Service {
	return &Service{repo: repo, graphene: g, access: access, library: lib, profiles: profiles, limits: limits, compiler: compiler, publisher: publisher, audit: auditSvc, scope: scope}
}

// Overrides are the launch-time deviations from a test.
type Overrides struct {
	Name              string
	ProviderProfileID *uuid.UUID
	Sizes             map[string]library.RoleSize
	Keep              *time.Duration
	RatingTenant      *bool
	RatingGlobal      *bool
	Labels            map[string]string
	Notes             string
	Trigger           Trigger
	// Suite / schedule provenance.
	SuiteRunID  *uuid.UUID
	CellID      string
	ScheduleID  *uuid.UUID
	ParentRunID *uuid.UUID
}

// Webhook event names.
const (
	EventRunStarted   = "run.started"
	EventRunFinished  = "run.finished"
	EventRunFailed    = "run.failed"
	EventRunCancelled = "run.cancelled"
)

// --- access -----------------------------------------------------------------

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
		return errs.Forbidden("viewers cannot launch or change runs")
	}
	return nil
}

// owned reads a run of the tenant.
func (s *Service) owned(ctx context.Context, tenantID, id uuid.UUID) (Run, error) {
	r, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Run{}, err
	}
	if r.TenantID != tenantID || r.DeletedAt != nil {
		return Run{}, errs.NotFound("run")
	}
	return r, nil
}

// --- launch -----------------------------------------------------------------

// Launch starts a run of a stored test.
func (s *Service) Launch(ctx context.Context, actor auth.Actor, tenantID, testID uuid.UUID, o Overrides, idempotencyKey string) (Run, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Run{}, err
	}
	if r, ok, err := s.replay(ctx, tenantID, idempotencyKey); err != nil || ok {
		return r, err
	}
	t, _, _, err := s.library.GetTest(ctx, actor, tenantID, testID)
	if err != nil {
		return Run{}, err
	}
	spec := t.Spec
	return s.launch(ctx, actor, tenantID, spec, &Ref{ID: t.ID, Name: t.Name}, o, idempotencyKey)
}

// LaunchSpec starts a run of an unsaved test spec (examples quick-run).
func (s *Service) LaunchSpec(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, spec library.TestSpec, o Overrides, idempotencyKey string) (Run, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Run{}, err
	}
	if r, ok, err := s.replay(ctx, tenantID, idempotencyKey); err != nil || ok {
		return r, err
	}
	return s.launch(ctx, actor, tenantID, spec, nil, o, idempotencyKey)
}

// Rerun starts a new run from a run's snapshot.
func (s *Service) Rerun(ctx context.Context, actor auth.Actor, tenantID, runID uuid.UUID, o Overrides, idempotencyKey string) (Run, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Run{}, err
	}
	if r, ok, err := s.replay(ctx, tenantID, idempotencyKey); err != nil || ok {
		return r, err
	}
	prev, err := s.owned(ctx, tenantID, runID)
	if err != nil {
		return Run{}, err
	}
	spec := prev.snapshotSpec()
	if o.ParentRunID == nil {
		o.ParentRunID = &prev.ID
	}
	if o.Name == "" {
		o.Name = prev.Name
	}
	var testRef *Ref
	if prev.TestID != nil {
		testRef = &Ref{ID: *prev.TestID, Name: prev.TestName}
	}
	return s.launch(ctx, actor, tenantID, spec, testRef, o, idempotencyKey)
}

// Resume reruns from the failed segment on the previous stand. Stand
// reuse needs the pipeline to attach foreign resources (open item 14 of
// the working doc); until then it degrades to a rerun and says so.
func (s *Service) Resume(ctx context.Context, actor auth.Actor, tenantID, runID uuid.UUID, fromSegment, idempotencyKey string) (Run, bool, error) {
	_ = fromSegment
	r, err := s.Rerun(ctx, actor, tenantID, runID, Overrides{Trigger: TriggerManual}, idempotencyKey)
	return r, false, err
}

// snapshotSpec rebuilds a test spec from the snapshot (inline copies, so
// a later library edit does not change what is rerun).
func (r Run) snapshotSpec() library.TestSpec {
	db, wl := r.Snapshot.Database, r.Snapshot.Workload
	spec := library.TestSpec{DatabaseInline: &db, WorkloadInline: &wl, Sizes: r.Snapshot.Sizes, Execution: r.Snapshot.Execution, Keep: r.Keep, RatingTenant: r.RatingTenant, RatingGlobal: r.RatingGlobal}
	if r.Snapshot.ProviderProfile.ID != uuid.Nil {
		id := r.Snapshot.ProviderProfile.ID
		spec.ProviderProfileID = &id
	}
	return spec
}

func (s *Service) replay(ctx context.Context, tenantID uuid.UUID, key string) (Run, bool, error) {
	if key == "" {
		return Run{}, false, nil
	}
	id, ok, err := s.repo.ByIdempotencyKey(ctx, tenantID, key)
	if err != nil || !ok {
		return Run{}, false, err
	}
	r, err := s.repo.ByID(ctx, id)
	return r, err == nil, err
}

// PrepareOptions are what a launch context (suite, schedule) adds.
type PrepareOptions struct {
	// GrapheneRunID is the id the run has on the Graphene side when it is
	// started by another pipeline (suite cells).
	GrapheneRunID string
	// ExtraLive counts sibling runs prepared in the same batch against
	// max_concurrent_runs.
	ExtraLive int
}

// Prepared is a run ready to be stored and started.
type Prepared struct {
	Run      Run
	Compiled Compiled
}

// launch is the common path: prepare, store, start.
func (s *Service) launch(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, spec library.TestSpec, testRef *Ref, o Overrides, idempotencyKey string) (Run, error) {
	p, err := s.Prepare(ctx, actor, tenantID, spec, testRef, o, PrepareOptions{})
	if err != nil {
		return Run{}, err
	}
	if err := s.repo.Insert(ctx, p.Run, idempotencyKey); err != nil {
		return Run{}, err
	}
	return s.Start(ctx, p.Run)
}

// Prepare resolves the test, checks fit/limits/quotas and compiles the
// RunSpec; nothing is stored. Suites prepare every cell before storing any.
//
//nolint:funlen // one linear checklist
func (s *Service) Prepare(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, spec library.TestSpec, testRef *Ref, o Overrides, opts PrepareOptions) (Prepared, error) {
	if o.ProviderProfileID != nil {
		spec.ProviderProfileID = o.ProviderProfileID
	}
	if len(o.Sizes) > 0 {
		sizes := map[string]library.RoleSize{}
		for role, rs := range spec.Sizes {
			sizes[role] = rs
		}
		for role, rs := range o.Sizes {
			sizes[role] = rs
		}
		spec.Sizes = sizes
	}
	if o.Keep != nil {
		spec.Keep = *o.Keep
	}
	if o.RatingTenant != nil {
		spec.RatingTenant = *o.RatingTenant
	}
	if o.RatingGlobal != nil {
		spec.RatingGlobal = *o.RatingGlobal
	}
	fit, res, err := s.library.Validate(ctx, tenantID, spec)
	if err != nil {
		return Prepared{}, err
	}
	if !fit.Fits {
		err := errs.Invalid("test does not fit")
		err.Validation = fit.Issues
		return Prepared{}, err
	}
	if res.Profile == nil || res.DatabaseDerived == nil || res.WorkloadDerived == nil {
		return Prepared{}, errs.Invalid("test is not fully resolved")
	}
	// Inline definitions have no library entry: the spec is the source.
	dbSpec, dbName := spec.DatabaseInline, ""
	if res.Database != nil {
		dbSpec, dbName = &res.Database.Spec, res.Database.Name
	}
	wlSpec, wlName := spec.WorkloadInline, ""
	if res.Workload != nil {
		wlSpec, wlName = &res.Workload.Spec, res.Workload.Name
	}
	if dbSpec == nil || wlSpec == nil {
		return Prepared{}, errs.Invalid("test is not fully resolved")
	}
	if res.Profile.Status != provider.StatusReady {
		return Prepared{}, errs.Invalid(fmt.Sprintf("provider profile %q is %s", res.Profile.Name, res.Profile.Status))
	}
	if err := s.checkLimits(ctx, tenantID, spec, res, opts.ExtraLive); err != nil {
		return Prepared{}, err
	}
	slug, ns, err := s.tenantScope(ctx, actor, tenantID)
	if err != nil {
		return Prepared{}, err
	}
	id := uuid.New()
	labels := map[string]string{"stroppy.io/tenant": slug, "stroppy.io/run": id.String()}
	for k, v := range o.Labels {
		labels[k] = v
	}
	compiled, err := s.compiler.Compile(ctx, CompileRequest{
		RunID: id, Tenant: slug, Namespace: ns, Database: *dbSpec, Derived: *res.DatabaseDerived, Workload: *wlSpec,
		Sizes: spec.Sizes, Execution: spec.Execution, Profile: *res.Profile, Keep: spec.Keep, Labels: labels,
	})
	if err != nil {
		return Prepared{}, err
	}
	var finalSpec pipelinespec.Run
	if err := json.Unmarshal(compiled.Spec, &finalSpec); err != nil {
		return Prepared{}, err
	}
	if err := s.checkQuotas(ctx, *res.Profile, finalSpec); err != nil {
		return Prepared{}, err
	}
	name := o.Name
	if name == "" && testRef != nil {
		name = testRef.Name
	}
	if name == "" {
		name = fmt.Sprintf("%s · %s", dbSpec.Kind, orDefault(wlName, string(wlSpec.Protocol)))
	}
	trigger := o.Trigger
	if trigger == "" {
		trigger = TriggerManual
		if actor.IsAPIToken() {
			trigger = TriggerAPI
		}
	}
	now := time.Now().UTC()
	r := Run{
		ID: id, TenantID: tenantID, Name: name, Status: StatusPending, Phase: PhaseQueued, Trigger: trigger,
		SuiteRunID: o.SuiteRunID, CellID: o.CellID, ScheduleID: o.ScheduleID, ParentRunID: o.ParentRunID,
		AuthorID: authorOf(actor), RunSpec: compiled.Spec, RatingTenant: spec.RatingTenant, RatingGlobal: spec.RatingGlobal,
		Keep: spec.Keep, Notes: o.Notes, Labels: o.Labels, GrapheneNamespace: ns, GrapheneRunID: opts.GrapheneRunID, CreatedAt: now, UpdatedAt: now,
	}
	if testRef != nil {
		r.TestID, r.TestName = &testRef.ID, testRef.Name
	}
	r.Snapshot = Snapshot{
		Database: *dbSpec, DatabaseName: dbName, Workload: *wlSpec, WorkloadName: wlName,
		Sizes: spec.Sizes, Execution: spec.Execution, ProviderProfile: Ref{ID: res.Profile.ID, Name: res.Profile.Name}, ProviderKind: string(res.Profile.Kind),
		EffectiveConfigs: res.DatabaseDerived.EffectiveConfigs, Machines: compiled.Machines, TopologyLabel: res.DatabaseDerived.Plan.Label,
	}
	if spec.Keep > 0 {
		r.Snapshot.Keep = spec.Keep.String()
	}
	sizes := map[string]string{}
	for role, rs := range spec.Sizes {
		sizes[role] = rs.Size
	}
	r.Summary = Summary{
		DBKind: string(dbSpec.Kind), DBVersion: dbSpec.Version, WorkloadName: wlName,
		Protocol: string(wlSpec.Protocol), StroppyVersion: wlSpec.StroppyVersion,
		TopologyLabel: res.DatabaseDerived.Plan.Label, NodeCount: len(compiled.Machines), ProviderKind: string(res.Profile.Kind),
		ProviderProfile: &Ref{ID: res.Profile.ID, Name: res.Profile.Name}, Sizes: sizes,
	}
	r.State = NewState(compiled.Machines, segmentNames(*wlSpec))
	return Prepared{Run: r, Compiled: compiled}, nil
}

// Insert stores a prepared run without starting it (suite cells: the
// suite pipeline starts them).
func (s *Service) Insert(ctx context.Context, r Run) error {
	return s.repo.Insert(ctx, r, "")
}

// Start hands a stored run to Graphene and records the outcome.
func (s *Service) Start(ctx context.Context, r Run) (Run, error) {
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &r.TenantID, Action: "run.launch", Target: audit.Target{Kind: "run", ID: r.ID.String(), Name: r.Name}, Details: map[string]any{"test": r.TestID, "trigger": r.Trigger}}) //nolint:errcheck // audit never blocks
	labels := map[string]string{"stroppy.io/run": r.ID.String()}
	for k, v := range r.Labels {
		labels[k] = v
	}
	if err := s.graphene.StartRun(s.scope(ctx, r.GrapheneNamespace), r.GrapheneID(), "stroppy-run", r.RunSpec, labels); err != nil {
		reason := "start: " + err.Error()
		_ = s.repo.SetStatus(ctx, r.ID, StatusFailed, PhaseDone, reason, nil, ptr(time.Now().UTC())) //nolint:errcheck // reported below
		return Run{}, errs.Wrap(errs.CodeUnavailable, "graphene did not accept the run", err)
	}
	s.publish(ctx, r.TenantID, EventRunStarted, r)
	return s.repo.ByID(ctx, r.ID)
}

// MarkStarted records that another pipeline started the run (suites).
func (s *Service) MarkStarted(ctx context.Context, r Run) {
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &r.TenantID, Action: "run.launch", Target: audit.Target{Kind: "run", ID: r.ID.String(), Name: r.Name}, Details: map[string]any{"suite_run": r.SuiteRunID, "cell": r.CellID}}) //nolint:errcheck // audit never blocks
	s.publish(ctx, r.TenantID, EventRunStarted, r)
}

func (s *Service) tenantScope(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, ns string, err error) {
	slug, _, err = s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return "", "", err
	}
	ns, err = s.access.NamespaceOf(ctx, tenantID)
	return slug, ns, err
}

func segmentNames(w library.WorkloadSpec) []string {
	out := make([]string, 0, len(w.Segments))
	for i, raw := range w.Segments {
		var seg struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(raw, &seg) //nolint:errcheck // baked upstream
		if seg.Name == "" {
			seg.Name = fmt.Sprintf("segment-%d", i+1)
		}
		out = append(out, seg.Name)
	}
	return out
}

// checkLimits enforces the tenant's ceiling (§16.4).
func (s *Service) checkLimits(ctx context.Context, tenantID uuid.UUID, spec library.TestSpec, res library.Resolved, extraLive int) error {
	lim, err := s.limits.EffectiveLimits(ctx, tenantID)
	if err != nil {
		return err
	}
	live, err := s.repo.LiveCount(ctx, tenantID)
	if err != nil {
		return err
	}
	live += extraLive
	if lim.MaxConcurrentRuns > 0 && live >= lim.MaxConcurrentRuns {
		return errs.Newf(errs.CodeLimit, "max_concurrent_runs: %d runs are live, limit %d", live, lim.MaxConcurrentRuns)
	}
	machines := 0
	for _, m := range res.Machines {
		machines += m.Count
	}
	if lim.MaxMachinesPerRun > 0 && machines > lim.MaxMachinesPerRun {
		return errs.Newf(errs.CodeLimit, "max_machines_per_run: %d machines, limit %d", machines, lim.MaxMachinesPerRun)
	}
	if lim.MaxSize != "" {
		for role, rs := range spec.Sizes {
			if sizeIndex(rs.Size) > sizeIndex(lim.MaxSize) {
				return errs.Newf(errs.CodeLimit, "max_size: sizes.%s is %s, limit %s", role, rs.Size, lim.MaxSize)
			}
		}
	}
	if lim.MaxKeep > 0 && spec.Keep > lim.MaxKeep {
		return errs.Newf(errs.CodeLimit, "max_keep: %s, limit %s", spec.Keep, lim.MaxKeep)
	}
	return nil
}

func sizeIndex(size string) int {
	for i, s := range catalog.Sizes {
		if s == size {
			return i
		}
	}
	return -1
}

// checkQuotas compares the final run's compute, storage and egress against fresh
// quotas; unknown quota names are ignored.
func (s *Service) checkQuotas(ctx context.Context, p provider.Profile, res pipelinespec.Run) error {
	quotas, err := s.profiles.FreshQuotas(ctx, p)
	if err != nil {
		return errs.Wrap(errs.CodeUnavailable, "provider quotas", err)
	}
	cpu, mem, highFreqCPU := 0, 0, 0
	for _, m := range res.Machines {
		cpu += m.CPU
		mem += m.MemoryGB
		if strings.HasPrefix(m.InstanceType, "highfreq-") {
			highFreqCPU += m.CPU
		}
	}
	storage := map[string]float64{}
	diskCount := 0
	for _, m := range res.Machines {
		boot := pipelinespec.BootDisk{GB: 40, Type: "network-ssd"}
		if res.Provider.Kind == pipelinespec.ProviderAWS {
			boot.Type = "gp3"
		}
		if m.BootDisk != nil {
			boot = *m.BootDisk
		}
		storage[boot.Type] += float64(boot.GB)
		diskCount++
		for _, d := range m.Disks {
			storage[d.Type] += float64(d.GB)
			diskCount++
		}
	}
	for _, q := range quotas {
		name := strings.ToLower(q.Name)
		var need float64
		switch {
		case name == "vpc.gateways.count" || name == "vpc.routetables.count":
			if pipelinespec.NeedsPrivateEgress(res) {
				need = 1
			}
		case name == "compute.disks.count":
			need = float64(diskCount)
		case name == "compute.instances.count":
			need = float64(len(res.Machines))
		case name == "compute.hdddisks.size":
			need = quotaStorageUnit(storage["network-hdd"], q.Unit)
		case name == "compute.ssddisks.size":
			need = quotaStorageUnit(storage["network-ssd"], q.Unit)
		case name == "compute.ssdnonreplicateddisks.size":
			need = quotaStorageUnit(storage["network-ssd-nonreplicated"], q.Unit)
		case name == "compute.ssdiom3disks.size":
			need = quotaStorageUnit(storage["network-ssd-io-m3"], q.Unit)
		case name == "compute.instancehighfreqcores.count":
			need = float64(highFreqCPU)
		case strings.Contains(name, "cpu") || strings.Contains(name, "cores"):
			need = float64(cpu)
		case strings.Contains(name, "memory") || strings.Contains(name, "ram"):
			need = float64(mem)
			if strings.Contains(strings.ToLower(q.Unit), "b") && !strings.Contains(strings.ToLower(q.Unit), "gb") && !strings.Contains(strings.ToLower(q.Unit), "gib") {
				need = float64(mem) * 1024 * 1024 * 1024
			}
		default:
			continue
		}
		if q.Limit >= 0 && need > 0 && q.Used+need > q.Limit {
			return errs.Newf(errs.CodeLimit, "provider quota %s: %.0f used + %.0f needed > %.0f %s", q.Name, q.Used, need, q.Limit, q.Unit)
		}
	}
	return nil
}

func (s *Service) publish(ctx context.Context, tenantID uuid.UUID, event string, r Run) {
	if s.publisher == nil {
		return
	}
	payload := map[string]any{"id": r.ID, "name": r.Name, "status": r.Status, "phase": r.Phase, "status_reason": r.StatusReason, "summary": r.Summary, "test_id": r.TestID, "suite_run_id": r.SuiteRunID}
	_ = s.publisher.Publish(ctx, tenantID, event, payload) //nolint:errcheck // delivery is best-effort
}

// --- reads ------------------------------------------------------------------

// Peek reads a run without an access check (the caller resolves access
// from the run's tenant).
func (s *Service) Peek(ctx context.Context, id uuid.UUID) (Run, error) {
	r, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Run{}, err
	}
	if r.DeletedAt != nil {
		return Run{}, errs.NotFound("run")
	}
	return r, nil
}

// Get reads one run (any member).
func (s *Service) Get(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Run, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Run{}, err
	}
	return s.owned(ctx, tenantID, id)
}

// List filters the tenant's runs; favorites resolve against the actor.
func (s *Service) List(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q ListQuery) ([]Run, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	if q.FavoritesOf != "" && actor.UserID != uuid.Nil {
		q.FavoritesOf = actor.UserID.String()
	}
	return s.repo.List(ctx, tenantID, q)
}

// Counts is the status breakdown (dashboard).
func (s *Service) Counts(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (Counts, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Counts{}, err
	}
	return s.repo.Counts(ctx, tenantID)
}

// Facets are the filter value counts.
func (s *Service) Facets(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) ([]Facet, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.Facets(ctx, tenantID)
}

// OfTest lists the runs of one test, newest first.
func (s *Service) OfTest(ctx context.Context, actor auth.Actor, tenantID, testID uuid.UUID, limit, offset int) ([]Run, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.OfTest(ctx, testID, limit, offset)
}

// Favorites of the actor by kind.
func (s *Service) Favorites(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, kind string) (map[uuid.UUID]bool, error) {
	if actor.UserID == uuid.Nil {
		return map[uuid.UUID]bool{}, nil
	}
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.FavoritesOf(ctx, actor.UserID, tenantID, kind)
}

// SetFavorite marks or unmarks a favorite of the actor.
func (s *Service) SetFavorite(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, kind string, target uuid.UUID, on bool) error {
	if actor.UserID == uuid.Nil {
		return errs.Forbidden("favorites belong to users, not tokens")
	}
	if err := s.member(ctx, actor, tenantID); err != nil {
		return err
	}
	return s.repo.SetFavorite(ctx, actor.UserID, tenantID, kind, target, on)
}

// Events pages the timeline after an event id.
func (s *Service) Events(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, after int64, limit int) ([]Event, error) {
	if _, err := s.Get(ctx, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.repo.EventsAfter(ctx, id, after, limit)
}

// Tree is the raw Graphene resource tree under the run. Graphene shows
// live records only: once the run hands its infrastructure to the stand
// (keep), the kept root is no longer under the run — it is added back
// here while it lives.
func (s *Service) Tree(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (TreeNode, error) {
	r, err := s.Get(ctx, actor, tenantID, id)
	if err != nil {
		return TreeNode{}, err
	}
	sctx := s.scope(ctx, r.GrapheneNamespace)
	tree, err := s.graphene.Tree(sctx, r.GrapheneRef())
	if err != nil {
		return TreeNode{}, err
	}
	// What the run kept moved to the pipeline's stand and left the run's
	// tree; the stand names this run as the holder, so it comes back here.
	if r.StandKept {
		held, err := s.graphene.Holdings(sctx, r.GrapheneRef())
		if err != nil {
			return TreeNode{}, err
		}
		for _, h := range held {
			node, ok, err := s.graphene.Node(sctx, h.Ref)
			if err != nil {
				return TreeNode{}, err
			}
			if ok {
				node.KeepUntil = h.KeepUntil
				tree.Children = append(tree.Children, node)
			}
		}
	}
	return tree, nil
}

// artifactRefs are the artifact records the run published: the pipeline
// names them in its result (they live under the stand, not the run).
func artifactRefs(r Run) []string {
	var res struct {
		Artifacts []string `json:"artifacts"`
	}
	if len(r.Result) == 0 || json.Unmarshal(r.Result, &res) != nil {
		return nil
	}
	return res.Artifacts
}

// Artifacts lists the run's downloadable outputs.
func (s *Service) Artifacts(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) ([]Artifact, error) {
	r, err := s.Get(ctx, actor, tenantID, id)
	if err != nil {
		return nil, err
	}
	refs := artifactRefs(r)
	if len(refs) == 0 {
		return []Artifact{}, nil
	}
	return s.graphene.Artifacts(s.scope(ctx, r.GrapheneNamespace), refs)
}

// Download streams one artifact; the ref must belong to the run.
func (s *Service) Download(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, ref string) (Artifact, io.ReadCloser, error) {
	r, err := s.Get(ctx, actor, tenantID, id)
	if err != nil {
		return Artifact{}, nil, err
	}
	arts, err := s.graphene.Artifacts(s.scope(ctx, r.GrapheneNamespace), artifactRefs(r))
	if err != nil {
		return Artifact{}, nil, err
	}
	for _, a := range arts {
		if a.Ref == ref || a.ID() == ref || a.Name == ref {
			rc, err := s.graphene.Download(s.scope(ctx, r.GrapheneNamespace), a.Ref)
			return a, rc, err
		}
	}
	return Artifact{}, nil, errs.NotFound("artifact")
}

// --- actions ----------------------------------------------------------------

// Patch changes the run's own metadata.
func (s *Service) Patch(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, p MetaPatch) (Run, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Run{}, err
	}
	if _, err := s.owned(ctx, tenantID, id); err != nil {
		return Run{}, err
	}
	if err := s.repo.UpdateMeta(ctx, id, p); err != nil {
		return Run{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "run.update", Target: audit.Target{Kind: "run", ID: id.String()}, Details: nil}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, id)
}

// Cancel asks Graphene to stop the run; the projector records the end.
func (s *Service) Cancel(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Run, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Run{}, err
	}
	r, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Run{}, err
	}
	if r.Status.Terminal() {
		return Run{}, errs.Conflict("run already finished")
	}
	// Mark first: the projector may record the terminal status the
	// moment Graphene acknowledges, and a terminal status never moves back.
	if err := s.repo.SetStatus(ctx, id, StatusCancelling, r.Phase, "cancel requested", nil, nil); err != nil {
		return Run{}, err
	}
	if err := s.graphene.CancelRun(s.scope(ctx, r.GrapheneNamespace), r.GrapheneID()); err != nil {
		return Run{}, errs.Wrap(errs.CodeUnavailable, "cancel", err)
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "run.cancel", Target: audit.Target{Kind: "run", ID: id.String()}, Details: nil}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, id)
}

// Delete soft-deletes the run and removes its Graphene resources.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return err
	}
	r, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !r.Status.Terminal() {
		return errs.Conflict("cancel the run before deleting it")
	}
	if err := s.graphene.DeleteRef(s.scope(ctx, r.GrapheneNamespace), r.GrapheneRef()); err != nil {
		return errs.Wrap(errs.CodeUnavailable, "delete graphene resources", err)
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "run.delete", Target: audit.Target{Kind: "run", ID: id.String()}, Details: nil}) //nolint:errcheck // audit never blocks
	return nil
}

// KeepExtend prolongs a kept stand.
func (s *Service) KeepExtend(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, d time.Duration) (Run, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Run{}, err
	}
	r, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Run{}, err
	}
	if !r.StandKept {
		return Run{}, errs.Conflict("the stand is not kept")
	}
	lim, err := s.limits.EffectiveLimits(ctx, tenantID)
	if err != nil {
		return Run{}, err
	}
	if lim.MaxKeep > 0 && d > lim.MaxKeep {
		return Run{}, errs.Newf(errs.CodeLimit, "max_keep: %s, limit %s", d, lim.MaxKeep)
	}
	sctx := s.scope(ctx, r.GrapheneNamespace)
	held, err := s.graphene.Holdings(sctx, r.GrapheneRef())
	if err != nil {
		return Run{}, errs.Wrap(errs.CodeUnavailable, "stand", err)
	}
	if len(held) == 0 {
		return Run{}, errs.Conflict("the stand holds nothing of this run any more")
	}
	for _, h := range held {
		if err := s.graphene.KeepExtend(sctx, h.Ref, d); err != nil {
			return Run{}, errs.Wrap(errs.CodeUnavailable, "extend", err)
		}
	}
	until := time.Now().UTC().Add(d)
	if err := s.repo.SetKeep(ctx, id, true, &until); err != nil {
		return Run{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "run.keep.extend", Target: audit.Target{Kind: "run", ID: id.String()}, Details: map[string]any{"duration": d.String()}}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, id)
}

// KeepRelease tears a kept stand down now.
func (s *Service) KeepRelease(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Run, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Run{}, err
	}
	r, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Run{}, err
	}
	if !r.StandKept {
		return Run{}, errs.Conflict("the stand is not kept")
	}
	sctx := s.scope(ctx, r.GrapheneNamespace)
	held, err := s.graphene.Holdings(sctx, r.GrapheneRef())
	if err != nil {
		return Run{}, errs.Wrap(errs.CodeUnavailable, "stand", err)
	}
	for _, h := range held {
		if err := s.graphene.KeepRelease(sctx, h.Ref); err != nil {
			return Run{}, errs.Wrap(errs.CodeUnavailable, "release", err)
		}
	}
	if err := s.repo.SetKeep(ctx, id, false, nil); err != nil {
		return Run{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "run.keep.release", Target: audit.Target{Kind: "run", ID: id.String()}, Details: nil}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, id)
}

// SaveAsTest stores the snapshot as a test (optionally its database and
// workload as library entries).
func (s *Service) SaveAsTest(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, name, dbName, wlName string) (library.Test, library.Fit, library.Resolved, error) {
	r, err := s.Get(ctx, actor, tenantID, id)
	if err != nil {
		return library.Test{}, library.Fit{}, library.Resolved{}, err
	}
	spec := r.snapshotSpec()
	if dbName != "" {
		db, _, err := s.library.CreateDatabase(ctx, actor, tenantID, library.EntityWrite{Name: dbName}, r.Snapshot.Database)
		if err != nil {
			return library.Test{}, library.Fit{}, library.Resolved{}, err
		}
		spec.DatabaseRef, spec.DatabaseInline = &db.ID, nil
	}
	if wlName != "" {
		wl, _, err := s.library.CreateWorkload(ctx, actor, tenantID, library.EntityWrite{Name: wlName}, r.Snapshot.Workload)
		if err != nil {
			return library.Test{}, library.Fit{}, library.Resolved{}, err
		}
		spec.WorkloadRef, spec.WorkloadInline = &wl.ID, nil
	}
	return s.library.CreateTest(ctx, actor, tenantID, library.EntityWrite{Name: name}, spec)
}

// --- helpers ----------------------------------------------------------------

// SortedPhases returns the phases in canonical order.
func (st State) SortedPhases() []PhaseState {
	out := append([]PhaseState(nil), st.Phases...)
	idx := map[Phase]int{}
	for i, p := range PhaseOrder {
		idx[p] = i
	}
	sort.SliceStable(out, func(i, j int) bool { return idx[out[i].ID] < idx[out[j].ID] })
	return out
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

func quotaStorageUnit(gb float64, unit string) float64 {
	u := strings.ToLower(unit)
	if u == "bytes" || u == "byte" || u == "b" {
		return gb * (1 << 30)
	}
	return gb
}
