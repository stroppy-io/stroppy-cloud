package library

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

/*
TEST = database + workload + sizes + provider. Validation is the product's
heart: does the workload speak the database's protocol, is every role
sized at least as big as it needs to be, is the provider profile ready,
does keep fit the limits. It never blocks a save — a test is a draft until
it fits — and it is recomputed on every read, so a changed library entry
or a re-verified profile is reflected without a stored cache.
*/

// sizeRank orders T-shirt sizes.
var sizeRank = map[string]int{"XS": 1, "S": 2, "M": 3, "L": 4, "XL": 5}

// CreateTest stores a test; status follows the validation (member+).
func (s *Service) CreateTest(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, w EntityWrite, spec TestSpec) (Test, Fit, Resolved, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	e, err := header(actor, tenantID, w)
	if err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	if err := s.normalizeTestSpec(ctx, &spec); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	t := Test{Entity: e, Spec: spec, Status: TestDraft}
	fit, res, err := s.Validate(ctx, tenantID, spec)
	if err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	t.Status = statusOf(fit)
	now := time.Now().UTC()
	t.ValidatedAt = &now
	if err := s.repo.InsertTest(ctx, t); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "test.create", Target: audit.Target{Kind: "test", ID: t.ID.String(), Name: t.Name}}) //nolint:errcheck // audit never blocks
	return t, fit, res, nil
}

// normalizeTestSpec bakes inline specs and checks references belong to
// the tenant. A pair with neither ref nor inline stays empty (draft).
func (s *Service) normalizeTestSpec(ctx context.Context, spec *TestSpec) error {
	if spec.DatabaseRef != nil && spec.DatabaseInline != nil {
		return errs.Invalid("database: ref and inline are exclusive")
	}
	if spec.WorkloadRef != nil && spec.WorkloadInline != nil {
		return errs.Invalid("workload: ref and inline are exclusive")
	}
	if spec.DatabaseInline != nil {
		baked, _, err := s.DeriveDatabase(ctx, *spec.DatabaseInline)
		if err != nil {
			return err
		}
		spec.DatabaseInline = &baked
	}
	if spec.WorkloadInline != nil {
		baked, _, _, err := s.DeriveWorkload(ctx, *spec.WorkloadInline)
		if err != nil {
			return err
		}
		spec.WorkloadInline = &baked
	}
	var err error
	spec.Execution, err = pipelinespec.NormalizeRuntime(spec.Execution)
	if err != nil {
		return pipelinespec.WithValidationPath(err, "execution")
	}
	for role, rs := range spec.Sizes {
		rs.Machine, err = pipelinespec.NormalizeMachineOverride(rs.Machine)
		if err != nil {
			return pipelinespec.WithValidationPath(err, "sizes."+role+".machine")
		}
		spec.Sizes[role] = rs
		if sizeRank[rs.Size] == 0 {
			return errs.Invalid(fmt.Sprintf("sizes.%s: unknown size %q", role, rs.Size))
		}
		if rs.DiskGB < 0 || rs.DiskGB > 262144 {
			return errs.Invalid(fmt.Sprintf("sizes.%s.disk.gb out of range", role))
		}
	}
	if spec.Keep < 0 {
		return errs.Invalid("keep must not be negative")
	}
	return nil
}

func statusOf(fit Fit) TestStatus {
	if fit.Fits {
		return TestReady
	}
	return TestDraft
}

// Validate resolves references and checks the fit of a spec (no access
// check: callers did it). The resolved view is returned even when the
// spec does not fit, so the form can show what it has.
func (s *Service) Validate(ctx context.Context, tenantID uuid.UUID, spec TestSpec) (Fit, Resolved, error) {
	fit := Fit{Issues: []Issue{}}
	res := Resolved{Requirements: map[string]topology.Requirement{}}
	issue := func(path, code, severity, msg string, suggested any) {
		fit.Issues = append(fit.Issues, Issue{Path: path, Code: code, Severity: severity, Message: msg, Suggested: suggested})
	}

	// database
	var dbSpec *DatabaseSpec
	switch {
	case spec.DatabaseRef != nil:
		d, err := s.repo.DatabaseByID(ctx, *spec.DatabaseRef)
		if err != nil || d.TenantID != tenantID {
			issue("database", "missing", "ERROR", "referenced database does not exist", nil)
		} else {
			res.Database = &d
			dbSpec = &d.Spec
		}
	case spec.DatabaseInline != nil:
		dbSpec = spec.DatabaseInline
	default:
		issue("database", "required", "ERROR", "choose a database", nil)
	}
	if dbSpec != nil {
		_, derived, err := s.DeriveDatabase(ctx, *dbSpec)
		if err != nil {
			issue("database", "invalid", "ERROR", err.Error(), nil)
		} else {
			res.DatabaseDerived = &derived
			for role, req := range derived.Plan.Requirements {
				res.Requirements[role] = req
			}
		}
	}

	// workload
	var wlSpec *WorkloadSpec
	switch {
	case spec.WorkloadRef != nil:
		w, err := s.repo.WorkloadByID(ctx, *spec.WorkloadRef)
		if err != nil || w.TenantID != tenantID {
			issue("workload", "missing", "ERROR", "referenced workload does not exist", nil)
		} else {
			res.Workload = &w
			wlSpec = &w.Spec
		}
	case spec.WorkloadInline != nil:
		wlSpec = spec.WorkloadInline
	default:
		issue("workload", "required", "ERROR", "choose a workload", nil)
	}
	if wlSpec != nil {
		_, _, derived, err := s.DeriveWorkload(ctx, *wlSpec)
		if err != nil {
			issue("workload", "invalid", "ERROR", err.Error(), nil)
		} else {
			res.WorkloadDerived = &derived
			res.Requirements[topology.RoleRunner] = derived.Runner
		}
	}

	// protocol match
	if dbSpec != nil && wlSpec != nil {
		if kind, ok := s.catalog.Database(dbSpec.Kind); ok && !hasProtocol(kind.Protocols, wlSpec.Protocol) {
			issue("workload.protocol", "protocol_mismatch", "ERROR",
				fmt.Sprintf("%s speaks %s, the workload uses %s", kind.Title, joinProtocols(kind.Protocols), wlSpec.Protocol), string(kind.Protocols[0]))
		}
	}

	// provider profile
	var sizes map[string]map[string]catalog.SizeSpec
	if spec.ProviderProfileID != nil {
		p, err := s.profiles.ByID(ctx, *spec.ProviderProfileID)
		if err != nil || p.TenantID != tenantID {
			issue("provider_profile_id", "missing", "ERROR", "provider profile does not exist", nil)
		} else {
			res.Profile = &p
			if p.Status != provider.StatusReady {
				issue("provider_profile_id", "not_ready", "ERROR", fmt.Sprintf("provider profile is %s", p.Status), nil)
			}
			if prov, ok := s.catalog.Provider(string(p.Kind)); ok {
				sizes = prov.Sizes
			}
		}
	} else {
		issue("provider_profile_id", "required", "ERROR", "choose a provider profile", nil)
	}
	if sizes == nil {
		// Without a provider the yandex table sizes the estimate, so the
		// form still shows numbers.
		if prov, ok := s.catalog.Provider(string(catalog.Yandex)); ok {
			sizes = prov.Sizes
		}
	}

	preview := pipelinespec.Run{}
	if wlSpec != nil {
		preview.Workload = pipelinespec.Workload{DriverType: string(wlSpec.Protocol), Segments: wlSpec.Segments}
	}
	if res.Profile != nil {
		preview.Provider.Kind = pipelinespec.ProviderKind(res.Profile.Kind)
	}
	// sizes vs requirements
	if res.DatabaseDerived != nil {
		roles := []string{}
		for _, n := range res.DatabaseDerived.Plan.Nodes {
			if n.ColocatedWith == "" {
				roles = append(roles, n.Role)
			}
		}
		for _, n := range res.DatabaseDerived.Plan.Nodes {
			if n.ColocatedWith != "" {
				continue
			}
			role := n.Role
			req := res.Requirements[role]
			rs, ok := spec.Sizes[role]
			if !ok {
				issue("sizes."+role, "required", "ERROR", "choose a size for "+role, suggestSize(sizes, role, req))
				continue
			}
			table := sizes[topology.Family(role)]
			cell, ok := table[rs.Size]
			if !ok {
				issue("sizes."+role, "unknown_size", "ERROR", fmt.Sprintf("no %s size for %s", rs.Size, topology.Family(role)), nil)
				continue
			}
			estimate := pipelinespec.DatasetEstimate{}
			if wlSpec != nil && wlSpec.Protocol != catalog.ProtoNoop {
				estimate = pipelinespec.EstimateDataset(wlSpec.Segments)
			}
			runtime := json.RawMessage(nil)
			if dbSpec != nil {
				runtime = dbSpec.Runtime
			}
			for index := 1; index <= n.Count; index++ {
				m, err := previewHardware(role, index, rs, cell, estimate, runtime, spec.Execution)
				if err != nil {
					issue("sizes."+role, "invalid_hardware", "ERROR", err.Error(), nil)
					continue
				}
				disk := 40
				if m.BootDisk != nil {
					disk = m.BootDisk.GB
				}
				for _, d := range m.Disks {
					if d.Mount == "/data" {
						disk = d.GB
					}
				}
				if m.CPU < req.CPU || float64(m.MemoryGB) < req.MemoryGB {
					issue("sizes."+role, "too_small", "ERROR", fmt.Sprintf("%s needs %d vCPU / %.0f GB (%s); final machine gives %d / %d", m.Name, req.CPU, req.MemoryGB, req.Reason, m.CPU, m.MemoryGB), suggestSize(sizes, role, req))
				}
				if float64(disk) < req.DiskGB {
					issue("sizes."+role+".disk", "too_small", "ERROR", fmt.Sprintf("%s needs %.0f GB of disk (%s)", m.Name, req.DiskGB, req.Reason), map[string]any{"gb": int(math.Ceil(req.DiskGB))})
				}
				res.Machines = append(res.Machines, Machine{Role: role, Count: 1, Size: rs.Size, CPU: m.CPU, MemoryGB: m.MemoryGB, DiskGB: disk})
				preview.Machines = append(preview.Machines, m)
				if role == "db" || strings.HasPrefix(role, "db-") {
					preview.Containers = append(preview.Containers, pipelinespec.Container{Name: m.Name, Role: role, Machine: m.Name, Image: string(dbSpec.Kind), Mounts: []pipelinespec.Mount{{Source: "/data", Target: "/data"}}})
				}
			}
		}
		for role := range spec.Sizes {
			if !contains(roles, role) {
				issue("sizes."+role, "unknown_role", "ERROR", "the topology has no role "+role, nil)
			}
		}
	}

	if s.preflight != nil && dbSpec != nil && wlSpec != nil && res.DatabaseDerived != nil && res.Profile != nil {
		findings, err := s.preflight(ctx, *dbSpec, *wlSpec, res, spec)
		if err != nil {
			issue("execution", "preflight", "ERROR", err.Error(), nil)
		}
		fit.Issues = append(fit.Issues, findings...)
	} else {
		for _, finding := range pipelinespec.CheckResources(preview) {
			issue(finding.Path, finding.Code, finding.Severity, finding.Message, nil)
		}
	}

	// keep vs limits
	if spec.Keep > 0 && s.limits != nil {
		maxKeep, err := s.limits.MaxKeep(ctx, tenantID)
		if err == nil && spec.Keep > maxKeep {
			issue("keep", "over_limit", "ERROR", fmt.Sprintf("keep exceeds the tenant limit %s", maxKeep), maxKeep.String())
		}
	}

	// stale: referenced entities changed after the test was validated is
	// the caller's to decide (needs the test); here the flags stay false.
	sort.SliceStable(fit.Issues, func(i, j int) bool { return fit.Issues[i].Path < fit.Issues[j].Path })
	fit.Fits = true
	for _, i := range fit.Issues {
		if i.Severity == "ERROR" {
			fit.Fits = false
			break
		}
	}
	return fit, res, nil
}

// suggestSize is the smallest size of the family that satisfies req.
func suggestSize(sizes map[string]map[string]catalog.SizeSpec, role string, req topology.Requirement) any {
	table := sizes[topology.Family(role)]
	for _, size := range catalog.Sizes {
		cell, ok := table[size]
		if !ok {
			continue
		}
		if cell.CPU >= req.CPU && float64(cell.MemoryGB) >= req.MemoryGB {
			out := map[string]any{"size": size}
			if float64(cell.DefaultDiskGB) < req.DiskGB {
				out["disk"] = map[string]any{"gb": int(math.Ceil(req.DiskGB))}
			}
			return out
		}
	}
	return nil
}

func joinProtocols(ps []catalog.Protocol) string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, string(p))
	}
	return strings.Join(out, "/")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// GetTest reads a test with its live validation (any member).
func (s *Service) GetTest(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Test, Fit, Resolved, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	t, err := s.repo.TestByID(ctx, id)
	if err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	if err := s.owned(tenantID, t.Entity, "test"); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	fit, res, err := s.Validate(ctx, tenantID, t.Spec)
	if err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	// Stale = a referenced entity changed after the last validation.
	if t.ValidatedAt != nil {
		if res.Database != nil && res.Database.UpdatedAt.After(*t.ValidatedAt) {
			fit.StaleDatabase = true
		}
		if res.Workload != nil && res.Workload.UpdatedAt.After(*t.ValidatedAt) {
			fit.StaleWorkload = true
		}
	}
	status := statusOf(fit)
	if t.Status == TestReady && (!fit.Fits || fit.StaleDatabase || fit.StaleWorkload) {
		status = TestNeedsAttention
	}
	t.Status = status
	return t, fit, res, nil
}

// ListTests lists tests (any member).
func (s *Service) ListTests(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q ListQuery) ([]Test, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.Tests(ctx, tenantID, q)
}

// UpdateTest patches a test and re-validates (member+). Wizard steps
// patch incrementally; the status follows the fit.
func (s *Service) UpdateTest(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, p TestPatch) (Test, Fit, Resolved, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	t, err := s.repo.TestByID(ctx, id)
	if err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	if err := s.owned(tenantID, t.Entity, "test"); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" || len(n) > 128 {
			return Test{}, Fit{}, Resolved{}, errs.Invalid("name must be 1..128 characters")
		}
		p.Name = &n
	}
	merged := t.Spec
	if p.Execution != nil {
		merged.Execution = p.Execution
	}
	if p.SetDatabase {
		merged.DatabaseRef, merged.DatabaseInline = p.DatabaseRef, p.DatabaseInline
	}
	if p.SetWorkload {
		merged.WorkloadRef, merged.WorkloadInline = p.WorkloadRef, p.WorkloadInline
	}
	if p.Sizes != nil {
		merged.Sizes = p.Sizes
	}
	if p.SetProvider {
		merged.ProviderProfileID = p.ProviderProfileID
	}
	if p.Keep != nil {
		merged.Keep = *p.Keep
	}
	if p.RatingTenant != nil {
		merged.RatingTenant = *p.RatingTenant
	}
	if p.RatingGlobal != nil {
		merged.RatingGlobal = *p.RatingGlobal
	}
	if err := s.normalizeTestSpec(ctx, &merged); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	p.DatabaseInline, p.WorkloadInline = merged.DatabaseInline, merged.WorkloadInline
	if p.Execution != nil {
		p.Execution = merged.Execution
	}
	fit, _, err := s.Validate(ctx, tenantID, merged)
	if err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	now := time.Now().UTC()
	t.Spec = merged
	t.Status = statusOf(fit)
	t.ValidatedAt = &now
	if err := s.repo.UpdateTest(ctx, t, p); err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "test.update", Target: audit.Target{Kind: "test", ID: id.String(), Name: t.Name}}) //nolint:errcheck // audit never blocks
	return s.GetTest(ctx, actor, tenantID, id)
}

// DeleteTest removes a test (runs keep their snapshots).
func (s *Service) DeleteTest(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return err
	}
	t, err := s.repo.TestByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.owned(tenantID, t.Entity, "test"); err != nil {
		return err
	}
	if s.testUsers != nil {
		usages, err := s.testUsers.UsingTest(ctx, id)
		if err != nil {
			return err
		}
		if len(usages) > 0 {
			names := make([]string, 0, len(usages))
			for _, u := range usages {
				names = append(names, u.Kind+" "+u.Name)
			}
			return errs.Conflict("test is used by " + strings.Join(names, ", "))
		}
	}
	if err := s.repo.DeleteTest(ctx, id); err != nil {
		return err
	}
	return s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "test.delete", Target: audit.Target{Kind: "test", ID: id.String(), Name: t.Name}})
}

// CloneTest copies a test under a new name.
func (s *Service) CloneTest(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, name string) (Test, Fit, Resolved, error) {
	src, _, _, err := s.GetTest(ctx, actor, tenantID, id)
	if err != nil {
		return Test{}, Fit{}, Resolved{}, err
	}
	if name == "" {
		name = src.Name + " (copy)"
	}
	return s.CreateTest(ctx, actor, tenantID, EntityWrite{Name: name, Description: src.Description, Tags: src.Tags}, src.Spec)
}

// ValidateSpec is the live-form validation (any member).
func (s *Service) ValidateSpec(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, spec TestSpec) (Fit, Resolved, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Fit{}, Resolved{}, err
	}
	if err := s.normalizeTestSpec(ctx, &spec); err != nil {
		return Fit{}, Resolved{}, err
	}
	return s.Validate(ctx, tenantID, spec)
}

// TestJSON is the spec as stored in exports and inline copies.
func (spec TestSpec) TestJSON() json.RawMessage {
	raw, _ := json.Marshal(spec) //nolint:errcheck // plain struct
	return raw
}
