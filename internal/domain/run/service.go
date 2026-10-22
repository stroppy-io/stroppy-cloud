package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
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
	compiler  Compiler
	publisher Publisher
	audit     *audit.Service
	// scope binds a Graphene call to a tenant namespace.
	scope func(ctx context.Context, namespace string) context.Context
	// The queue (queue.go).
	pipelines PipelineReadiness
	suites    SuiteConcurrency
	log       *xlog.Logger
	nudge     chan struct{}
}

// NewService wires the use cases.
func NewService(repo Repository, g Graphene, access Access, lib *library.Service, profiles Profiles, compiler Compiler, publisher Publisher, auditSvc *audit.Service, scope func(context.Context, string) context.Context) *Service {
	return &Service{repo: repo, graphene: g, access: access, library: lib, profiles: profiles, compiler: compiler, publisher: publisher, audit: auditSvc, scope: scope, nudge: make(chan struct{}, 1)}
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
	// Resume asks to continue on the previous run's kept stand: the same
	// machines, the workload again. A stand that is gone (or a rerun that
	// changes the machines) degrades to a full rerun and says so in the
	// notes.
	Resume bool
	// DatabaseName and WorkloadName name inline definitions in the
	// snapshot (a rerun keeps the names its source run showed).
	DatabaseName string
	WorkloadName string
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
		return errs.Forbidden("viewers cannot launch or change runs").Because("viewers_cannot_launch_or_change_runs")
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
	if o.DatabaseName == "" {
		o.DatabaseName = prev.Snapshot.DatabaseName
	}
	if o.WorkloadName == "" {
		o.WorkloadName = prev.Snapshot.WorkloadName
	}
	var testRef *Ref
	if prev.TestID != nil {
		testRef = &Ref{ID: *prev.TestID, Name: prev.TestName}
	}
	if o.Resume {
		why := ""
		switch {
		case !prev.StandKept || prev.KeepUntil == nil || !prev.KeepUntil.After(time.Now()):
			why = "the stand of " + prev.Name + " is gone"
		case o.ProviderProfileID != nil || len(o.Sizes) > 0:
			why = "a different provider or sizes need new machines"
		}
		if why == "" {
			return s.launchOn(ctx, actor, tenantID, spec, testRef, o, idempotencyKey, &prev)
		}
		note := "Resume requested but " + why + " — degraded to a full rerun."
		if o.Notes != "" {
			note = o.Notes + "\n\n" + note
		}
		o.Notes = note
	}
	return s.launch(ctx, actor, tenantID, spec, testRef, o, idempotencyKey)
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
	// GrapheneRunID is the id the run has on the Graphene side when it
	// differs from the row id (suite cells).
	GrapheneRunID string
	// Stand is the run whose kept stand this run continues on (resume): no
	// machines are created, so quotas are not checked and keep is off.
	Stand *Run
}

// Prepared is a run ready to be stored and started.
type Prepared struct {
	Run      Run
	Compiled Compiled
}

// launch is the common path: prepare, store, start.
func (s *Service) launch(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, spec library.TestSpec, testRef *Ref, o Overrides, idempotencyKey string) (Run, error) {
	return s.launchOn(ctx, actor, tenantID, spec, testRef, o, idempotencyKey, nil)
}

// launchOn launches on a kept stand when stand is set (resume).
func (s *Service) launchOn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, spec library.TestSpec, testRef *Ref, o Overrides, idempotencyKey string, stand *Run) (Run, error) {
	var admitted Run
	replayed := false
	err := s.repo.Admit(ctx, tenantID, func(ctx context.Context) error {
		if r, ok, err := s.replay(ctx, tenantID, idempotencyKey); err != nil || ok {
			admitted, replayed = r, ok
			return err
		}
		p, err := s.Prepare(ctx, actor, tenantID, spec, testRef, o, PrepareOptions{Stand: stand})
		if err != nil {
			return err
		}
		admitted = p.Run
		return s.repo.Insert(ctx, p.Run, idempotencyKey)
	})
	if err != nil {
		return Run{}, err
	}
	if replayed {
		return admitted, nil
	}
	s.admitTenantNow(ctx, tenantID)
	return s.repo.ByID(ctx, admitted.ID)
}

// Prepare resolves the test, checks fit/quotas and compiles the
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
	if opts.Stand != nil {
		spec.Keep = 0
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
		err := errs.Invalid("test does not fit").Because("test_does_not_fit")
		err.Validation = fit.Issues
		return Prepared{}, err
	}
	if res.Profile == nil || res.DatabaseDerived == nil || res.WorkloadDerived == nil {
		return Prepared{}, errs.Invalid("test is not fully resolved").Because("test_is_not_fully_resolved")
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
		return Prepared{}, errs.Invalid("test is not fully resolved").Because("test_is_not_fully_resolved")
	}
	// A rerun's inline copies keep the names its source run showed.
	if o.DatabaseName != "" {
		dbName = o.DatabaseName
	}
	if o.WorkloadName != "" {
		wlName = o.WorkloadName
	}
	if res.Profile.Status != provider.StatusReady {
		return Prepared{}, errs.Invalid(fmt.Sprintf("provider profile %q is %s", res.Profile.Name, res.Profile.Status)).Because("profile_not_ready", "name", res.Profile.Name, "status", string(res.Profile.Status))
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
	if opts.Stand != nil {
		if finalSpec.Resume, err = resumeOf(*opts.Stand, finalSpec); err != nil {
			return Prepared{}, err
		}
		if compiled.Spec, err = json.Marshal(finalSpec); err != nil {
			return Prepared{}, err
		}
	} else if err := s.checkQuotas(ctx, *res.Profile, finalSpec); err != nil {
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
		EffectiveConfigs: res.DatabaseDerived.EffectiveConfigs, Machines: compiled.Machines, Topology: res.DatabaseDerived.Plan.Shape(),
	}
	if compiled.EffectiveConfigs != nil {
		r.Snapshot.EffectiveConfigs = compiled.EffectiveConfigs
	}
	r.Snapshot.Tuning = compiled.Tuning
	if spec.Keep > 0 {
		r.Snapshot.Keep = spec.Keep.String()
	}
	sizes := map[string]string{}
	for role, rs := range spec.Sizes {
		sizes[role] = rs.Size
	}
	r.Summary = Summary{
		DBKind: string(dbSpec.Kind), DBVersion: dbSpec.Version, WorkloadName: workloadLabel(wlName, *wlSpec),
		Protocol: string(wlSpec.Protocol), StroppyVersion: wlSpec.StroppyVersion,
		Topology: res.DatabaseDerived.Plan.Shape(), NodeCount: len(compiled.Machines), ProviderKind: string(res.Profile.Kind),
		ProviderProfile: &Ref{ID: res.Profile.ID, Name: res.Profile.Name}, Sizes: sizes,
	}
	r.State = NewState(compiled.Machines, segmentNames(*wlSpec))
	return Prepared{Run: r, Compiled: compiled}, nil
}

// resumeOf points a compiled run at the stand another run kept: the same
// machines by name and role, with the addresses that run recorded.
func resumeOf(stand Run, next pipelinespec.Run) (*pipelinespec.Resume, error) {
	var prev pipelinespec.Run
	if err := json.Unmarshal(stand.RunSpec, &prev); err != nil || prev.RunID == "" {
		return nil, errs.Conflict("the kept stand has no run spec").Because("resume_stand_unknown")
	}
	kept := prev.MachineByName()
	out := &pipelinespec.Resume{FromRunID: prev.RunID, Machines: map[string]pipelinespec.ResumeMachine{}}
	for _, m := range next.Machines {
		was, ok := kept[m.Name]
		st := stand.State.Machines[m.Name]
		if !ok || was.Role != m.Role || st.PrivateIP == "" {
			return nil, errs.Conflict("the kept stand has no machine "+m.Name).Because("resume_machine_missing", "machine", m.Name)
		}
		out.Machines[m.Name] = pipelinespec.ResumeMachine{PrivateIP: st.PrivateIP, PublicIP: st.PublicIP}
	}
	return out, nil
}

// Insert stores a prepared run queued (suite cells); the queue starts it.
func (s *Service) Insert(ctx context.Context, r Run) error {
	return s.repo.Insert(ctx, r, "")
}

// AdmitTenant runs the queue over one tenant now (after a batch of
// inserts, such as a suite's cells).
func (s *Service) AdmitTenant(ctx context.Context, tenantID uuid.UUID) {
	s.admitTenantNow(ctx, tenantID)
}

// Start hands a stored run to Graphene and records the outcome.
func (s *Service) Start(ctx context.Context, r Run) (Run, error) {
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &r.TenantID, Action: "run.launch", Target: audit.Target{Kind: "run", ID: r.ID.String(), Name: r.Name}, Details: map[string]any{"test": r.TestID, "trigger": r.Trigger}}) //nolint:errcheck // audit never blocks
	if err := Submit(s.scope(ctx, r.GrapheneNamespace), s.graphene, r.submittedAt(), r.GrapheneID(), "stroppy-run", r.RunSpec, runLabels(r)); err != nil {
		if errors.Is(err, ErrStartRejected) {
			_ = s.repo.SetStatus(ctx, r.ID, StatusFailed, PhaseDone, "start: "+err.Error(), nil, ptr(time.Now().UTC())) //nolint:errcheck // reported below
			return Run{}, errs.Wrap(errs.CodeUnavailable, "graphene rejected the run", err).Because("run_submit_unavailable")
		}
		// The durable row is the accepted command. A lost answer does not
		// mean rejection; the projector submits it after connection recovery.
		return r, nil
	}
	s.publish(ctx, r.TenantID, EventRunStarted, r)
	return s.repo.ByID(ctx, r.ID)
}

func (s *Service) tenantScope(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, ns string, err error) {
	slug, _, err = s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return "", "", err
	}
	ns, err = s.access.NamespaceOf(ctx, tenantID)
	return slug, ns, err
}

// workloadLabel is the run's workload as lists show it: the referenced
// definition's name, else the first segment script with "+N" for the
// other distinct scripts, else the first segment name.
func workloadLabel(name string, w library.WorkloadSpec) string {
	if name != "" {
		return name
	}
	var scripts []string
	first := ""
	for _, raw := range w.Segments {
		var seg struct {
			Name     string `json:"name"`
			Workload struct {
				Script string `json:"script"`
			} `json:"workload"`
		}
		_ = json.Unmarshal(raw, &seg) //nolint:errcheck // baked upstream
		if first == "" {
			first = seg.Name
		}
		if seg.Workload.Script != "" && !slices.Contains(scripts, seg.Workload.Script) {
			scripts = append(scripts, seg.Workload.Script)
		}
	}
	switch len(scripts) {
	case 0:
		return first
	case 1:
		return scripts[0]
	default:
		return fmt.Sprintf("%s +%d", scripts[0], len(scripts)-1)
	}
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

// checkQuotas refuses a run that can never fit: it needs more than a
// provider quota's whole limit. A run that only waits for room is queued.
func (s *Service) checkQuotas(ctx context.Context, p provider.Profile, res pipelinespec.Run) error {
	quotas, err := s.profiles.FreshQuotas(ctx, p)
	if err != nil {
		return errs.Wrap(errs.CodeUnavailable, "provider quotas", err).Because("provider_quota_check_unavailable")
	}
	sf, never := quotaShortfall(quotas, nil, res)
	if !never {
		return nil
	}
	q := sf.quota
	name := q.Name
	if q.Zone != "" {
		name += " (" + q.Zone + ")"
	}
	return errs.Newf(errs.CodeLimit, "provider quota %s: %.0f needed > %.0f %s limit", name, sf.need, q.Limit, q.Unit).Because("provider_quota_exceeded", "quota", q.Name, "zone", q.Zone, "used", strconv.FormatFloat(q.Used, 'f', 0, 64), "need", strconv.FormatFloat(sf.need, 'f', 0, 64), "limit", strconv.FormatFloat(q.Limit, 'f', 0, 64), "unit", q.Unit)
}

// shortfall is the first quota a run does not fit into.
type shortfall struct {
	quota      provider.Quota
	need, free float64
}

// quotaShortfall finds the first quota without room for res once the held
// runs (admitted, machines not created yet, so not in the cloud's usage)
// take their share; never is set when res exceeds the whole limit. Unknown
// quota names are ignored; a zonal quota counts the machines of its zone.
func quotaShortfall(quotas []provider.Quota, held []pipelinespec.Run, res pipelinespec.Run) (sf *shortfall, never bool) {
	total, byZone := runDemand(res)
	type demands struct {
		total  demand
		byZone map[string]demand
	}
	heldDemand := make([]demands, len(held))
	for i, h := range held {
		heldDemand[i].total, heldDemand[i].byZone = runDemand(h)
	}
	for _, q := range quotas {
		d := total
		if q.Zone != "" {
			d = byZone[q.Zone]
		}
		need, ok := d.need(q, res)
		if !ok || q.Limit < 0 || need <= 0 {
			continue
		}
		if need > q.Limit {
			return &shortfall{quota: q, need: need, free: max(0, q.Limit-q.Used)}, true
		}
		taken := 0.0
		for i, h := range held {
			hd := heldDemand[i].total
			if q.Zone != "" {
				hd = heldDemand[i].byZone[q.Zone]
			}
			n, _ := hd.need(q, h)
			taken += n
		}
		if q.Used+taken+need > q.Limit {
			return &shortfall{quota: q, need: need, free: max(0, q.Limit-q.Used-taken)}, false
		}
	}
	return nil, false
}

// demand is what a set of machines takes from the provider.
type demand struct {
	machines, cpu, mem, highFreqCPU, disks int
	storage                                map[string]float64
}

// runDemand sums the run's machines, in total and per zone (a machine
// without a location lands in the provider's default zone).
func runDemand(res pipelinespec.Run) (total demand, byZone map[string]demand) {
	var settings struct {
		Zone string `json:"zone"`
	}
	_ = json.Unmarshal(res.Provider.Settings, &settings) //nolint:errcheck // no zone: only cloud-wide quotas apply
	total = demand{storage: map[string]float64{}}
	byZone = map[string]demand{}
	for _, m := range res.Machines {
		zone := m.Location
		if zone == "" {
			zone = settings.Zone
		}
		z, ok := byZone[zone]
		if !ok {
			z = demand{storage: map[string]float64{}}
		}
		total.add(m, res.Provider.Kind)
		z.add(m, res.Provider.Kind)
		byZone[zone] = z
	}
	return total, byZone
}

func (d *demand) add(m pipelinespec.Machine, kind pipelinespec.ProviderKind) {
	d.machines++
	d.cpu += m.CPU
	d.mem += m.MemoryGB
	if strings.HasPrefix(m.InstanceType, "highfreq-") {
		d.highFreqCPU += m.CPU
	}
	boot := pipelinespec.BootDisk{GB: 40, Type: "network-ssd"}
	if kind == pipelinespec.ProviderAWS {
		boot.Type = "gp3"
	}
	if m.BootDisk != nil {
		boot = *m.BootDisk
	}
	d.storage[boot.Type] += float64(boot.GB)
	d.disks++
	for _, disk := range m.Disks {
		d.storage[disk.Type] += float64(disk.GB)
		d.disks++
	}
}

// need is how much of the quota the machines take; false for a quota the
// check does not know.
func (d demand) need(q provider.Quota, res pipelinespec.Run) (float64, bool) {
	name := strings.ToLower(q.Name)
	switch {
	case name == "vpc.gateways.count" || name == "vpc.routetables.count":
		if pipelinespec.NeedsPrivateEgress(res) {
			return 1, true
		}
		return 0, true
	// Every run declares its own network and security group: one VPC,
	// one group and (AWS) one internet gateway per run, one subnet per
	// zone it places machines in (pipelines/internal/provision).
	case name == "vpc.networks.count" || name == "vpc.vpcs" ||
		name == "vpc.securitygroups.count" || name == "vpc.security_groups" ||
		name == "vpc.internet_gateways":
		return 1, true
	case name == "vpc.subnets.count":
		return float64(subnetCount(res)), true
	case name == "compute.disks.count":
		return float64(d.disks), true
	case name == "compute.instances.count":
		return float64(d.machines), true
	case name == "compute.hdddisks.size":
		return quotaStorageUnit(d.storage["network-hdd"], q.Unit), true
	case name == "compute.ssddisks.size":
		return quotaStorageUnit(d.storage["network-ssd"], q.Unit), true
	case name == "compute.ssdnonreplicateddisks.size":
		return quotaStorageUnit(d.storage["network-ssd-nonreplicated"], q.Unit), true
	case name == "compute.ssdiom3disks.size":
		return quotaStorageUnit(d.storage["network-ssd-io-m3"], q.Unit), true
	case name == "compute.instancehighfreqcores.count":
		return float64(d.highFreqCPU), true
	case strings.Contains(name, "cpu") || strings.Contains(name, "cores"):
		return float64(d.cpu), true
	case strings.Contains(name, "memory") || strings.Contains(name, "ram"):
		unit := strings.ToLower(q.Unit)
		if strings.Contains(unit, "b") && !strings.Contains(unit, "gb") && !strings.Contains(unit, "gib") {
			return float64(d.mem) * 1024 * 1024 * 1024, true
		}
		return float64(d.mem), true
	}
	return 0, false
}

// subnetCount is the zones the run places machines (and a dedicated
// managed YDB) in: one subnet each.
func subnetCount(res pipelinespec.Run) int {
	var settings struct {
		Zone string `json:"zone"`
	}
	_ = json.Unmarshal(res.Provider.Settings, &settings) //nolint:errcheck // no zone: the default one
	zones := map[string]bool{}
	for _, m := range res.Machines {
		zone := m.Location
		if zone == "" {
			zone = settings.Zone
		}
		zones[zone] = true
	}
	if res.ManagedYDB != nil && res.ManagedYDB.Type == "dedicated" {
		for _, zone := range res.ManagedYDB.Zones {
			zones[zone] = true
		}
	}
	return max(len(zones), 1)
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
	if actor.UserID != uuid.Nil {
		q.ViewerID = actor.UserID.String()
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

// TestRunStats — latest run + run count for a set of tests (member+).
func (s *Service) TestRunStats(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, testIDs []uuid.UUID) (map[uuid.UUID]TestRunStat, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	if len(testIDs) == 0 {
		return map[uuid.UUID]TestRunStat{}, nil
	}
	return s.repo.TestRunStats(ctx, testIDs)
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
		return errs.Forbidden("favorites belong to users, not tokens").Because("favorites_belong_to_users_not_tokens")
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
	// tree; the stand names this run as the holder, so it comes back
	// here — the kept infrastructure and the artifacts alike, because
	// the tree is what the run still has.
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

// standHoldings are the infrastructure holdings of a run: what keep
// extends and what release tears down. The run's artifacts sit on the
// same stand so they outlive the run, but they are downloads, not a
// stand — releasing the stand must not destroy what the run produced.
func standHoldings(held []Holding) []Holding {
	out := make([]Holding, 0, len(held))
	for _, h := range held {
		if strings.HasPrefix(h.Ref, "artifact/") {
			continue
		}
		out = append(out, h)
	}
	return out
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
		return Run{}, errs.Conflict("run already finished").Because("run_already_finished")
	}
	if err := s.CancelRun(ctx, r, "cancel requested"); err != nil {
		return Run{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "run.cancel", Target: audit.Target{Kind: "run", ID: id.String()}, Details: nil}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, id)
}

// CancelRun stops a run without an access check (the caller checked it, as
// a suite cancelling its cells): a queued run is cancelled at once, a
// started one is asked to stop in Graphene.
func (s *Service) CancelRun(ctx context.Context, r Run, reason string) error {
	if r.Queued() {
		ok, err := s.repo.CancelQueued(ctx, r.ID, reason)
		if err != nil {
			return err
		}
		if ok {
			cur, err := s.repo.ByID(ctx, r.ID)
			if err == nil {
				s.publish(ctx, r.TenantID, EventRunCancelled, cur)
			}
			s.Nudge()
			return nil
		}
		// Admitted meanwhile: cancel it as a started run.
		if r, err = s.repo.ByID(ctx, r.ID); err != nil {
			return err
		}
		if r.Status.Terminal() {
			return nil
		}
	}
	// Mark first: the projector may record the terminal status the
	// moment Graphene acknowledges, and a terminal status never moves back.
	if err := s.repo.SetStatus(ctx, r.ID, StatusCancelling, r.Phase, reason, nil, nil); err != nil {
		return err
	}
	if err := s.graphene.CancelRun(s.scope(ctx, r.GrapheneNamespace), r.GrapheneID()); err != nil {
		return errs.Wrap(errs.CodeUnavailable, "cancel", err).Because("cancel_unavailable")
	}
	return nil
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
		return errs.Conflict("cancel the run before deleting it").Because("cancel_run_before_delete")
	}
	if err := s.graphene.DeleteRef(s.scope(ctx, r.GrapheneNamespace), r.GrapheneRef()); err != nil {
		return errs.Wrap(errs.CodeUnavailable, "delete graphene resources", err).Because("delete_resources_unavailable")
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
		return Run{}, errs.Conflict("the stand is not kept").Because("the_stand_is_not_kept")
	}
	sctx := s.scope(ctx, r.GrapheneNamespace)
	held, err := s.graphene.Holdings(sctx, r.GrapheneRef())
	if err != nil {
		return Run{}, errs.Wrap(errs.CodeUnavailable, "stand", err).Because("stand_unavailable")
	}
	held = standHoldings(held)
	if len(held) == 0 {
		return Run{}, errs.Conflict("the stand holds nothing of this run any more").Because("stand_lost_run")
	}
	for _, h := range held {
		if err := s.graphene.KeepExtend(sctx, h.Ref, d); err != nil {
			return Run{}, errs.Wrap(errs.CodeUnavailable, "extend", err).Because("stand_unavailable")
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
		return Run{}, errs.Conflict("the stand is not kept").Because("the_stand_is_not_kept")
	}
	sctx := s.scope(ctx, r.GrapheneNamespace)
	held, err := s.graphene.Holdings(sctx, r.GrapheneRef())
	if err != nil {
		return Run{}, errs.Wrap(errs.CodeUnavailable, "stand", err).Because("stand_unavailable")
	}
	for _, h := range standHoldings(held) {
		if err := s.graphene.KeepRelease(sctx, h.Ref); err != nil {
			return Run{}, errs.Wrap(errs.CodeUnavailable, "release", err).Because("stand_unavailable")
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
