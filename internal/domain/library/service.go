package library

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// Service is the library use cases.
type Service struct {
	repo     Repository
	validate Validator
	catalog  *catalog.Catalog
	access   Access
	profiles Profiles
	limits   Limits
	audit    *audit.Service
	// testUsers reports who references a test beyond the library (suites,
	// schedules); nil = nobody.
	testUsers TestUsers
}

// TestUsers is the port other domains implement to report test usages.
type TestUsers interface {
	UsingTest(ctx context.Context, testID uuid.UUID) ([]Usage, error)
}

// WithTestUsers registers a usage reporter.
func (s *Service) WithTestUsers(u TestUsers) *Service {
	s.testUsers = u
	return s
}

// NewService builds the service.
func NewService(repo Repository, validate Validator, cat *catalog.Catalog, access Access, profiles Profiles, limits Limits, auditSvc *audit.Service) *Service {
	return &Service{repo: repo, validate: validate, catalog: cat, access: access, profiles: profiles, limits: limits, audit: auditSvc}
}

// member / writer are the two access levels: viewers read, members write.
func (s *Service) member(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, _, err := s.access.RoleIn(ctx, actor, tenantID)
	return err
}

func (s *Service) writer(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, role, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return err
	}
	if role == "viewer" {
		return errs.Forbidden("requires role member")
	}
	return nil
}

func header(actor auth.Actor, tenantID uuid.UUID, w EntityWrite) (Entity, error) {
	name := strings.TrimSpace(w.Name)
	if name == "" || len(name) > 128 {
		return Entity{}, errs.Invalid("name must be 1..128 characters")
	}
	e := Entity{ID: uuid.New(), TenantID: tenantID, Name: name, Description: strings.TrimSpace(w.Description), Tags: w.Tags, CreatedAt: time.Now().UTC()}
	if e.Tags == nil {
		e.Tags = map[string]string{}
	}
	if !actor.IsAPIToken() || actor.UserID != uuid.Nil {
		id := actor.UserID
		if id != uuid.Nil {
			e.AuthorID = &id
		}
	}
	return e, nil
}

func (s *Service) owned(tenantID uuid.UUID, e Entity, what string) error {
	if e.TenantID != tenantID {
		return errs.NotFound(what)
	}
	return nil
}

// --- databases --------------------------------------------------------------

// DeriveDatabase computes what a database spec means; the schema result of
// the params comes back as a validation error (CodeValidation) when they
// do not fit.
func (s *Service) DeriveDatabase(ctx context.Context, spec DatabaseSpec) (DatabaseSpec, DatabaseDerived, error) {
	kind, ok := s.catalog.Database(spec.Kind)
	if !ok {
		return spec, DatabaseDerived{}, errs.Invalid(fmt.Sprintf("unknown database kind %q", spec.Kind))
	}
	if !versionKnown(kind, spec.Version) {
		return spec, DatabaseDerived{}, errs.Invalid(fmt.Sprintf("%s has no version %q", spec.Kind, spec.Version))
	}
	if len(spec.Params) == 0 {
		spec.Params = json.RawMessage("{}")
	}
	baked, err := s.validate.Bake(ctx, kind.ParamsSchema, spec.Params)
	if err != nil {
		return spec, DatabaseDerived{}, err
	}
	spec.Params = baked
	var params map[string]any
	_ = json.Unmarshal(baked, &params) //nolint:errcheck // baked by us
	plan, err := topology.Compile(spec.Kind, params)
	if err != nil {
		return spec, DatabaseDerived{}, errs.Wrap(errs.CodeInvalid, "topology", err)
	}
	effective := map[string]map[string]json.RawMessage{}
	for _, role := range kind.Roles {
		for id := range spec.Configs[role.Role] {
			if resolved := kind.ConfigSchema(spec.Version, id); resolved != id {
				return spec, DatabaseDerived{}, errs.Invalid(fmt.Sprintf("configs.%s.%s: version %s requires %s", role.Role, id, spec.Version, resolved))
			}
		}
		for _, templateID := range role.ConfigSchemas {
			schemaID := kind.ConfigSchema(spec.Version, templateID)
			seed := role.Seed(templateID)
			// Cluster presets read through replicas. Seed causal reads before
			// user overrides, so explicit consistency experiments remain possible
			// and the UI preview shows the same settings as the compiler.
			if params["replication"] == "group" && strings.HasPrefix(schemaID, "cfg.my.cnf@") {
				seed["group_replication_consistency"] = "BEFORE"
			}
			if params["replication"] == "galera" && strings.HasPrefix(schemaID, "cfg.mariadb.cnf@") {
				seed["wsrep_sync_wait"] = 1
			}
			if params["replication"] == "galera" && strings.HasPrefix(schemaID, "cfg.proxysql.cnf@") {
				// Galera has no read-only members: populate the reader pool
				// from backup writers, while retaining explicit user overrides.
				seed["writer_is_also_reader"] = 2
			}
			if rc, ok := spec.Configs[role.Role]; ok {
				if v, ok := rc[schemaID]; ok && len(v) > 0 {
					user := map[string]any{}
					if err := json.Unmarshal(v, &user); err != nil {
						return spec, DatabaseDerived{}, errs.Wrap(errs.CodeInvalid, fmt.Sprintf("configs.%s.%s: not an object", role.Role, schemaID), err)
					}
					for k, x := range user {
						seed[k] = x
					}
				}
			}
			diff, _ := json.Marshal(seed) //nolint:errcheck // map of JSON values
			full, err := s.validate.Bake(ctx, schemaID, diff)
			if err != nil {
				if e, ok := errs.AsValidation(err); ok {
					e.Detail = fmt.Sprintf("configs.%s.%s: %s", role.Role, schemaID, e.Detail)
				}
				return spec, DatabaseDerived{}, err
			}
			if effective[role.Role] == nil {
				effective[role.Role] = map[string]json.RawMessage{}
			}
			effective[role.Role][schemaID] = full
		}
	}
	if spec.Kind == catalog.External && spec.ExternalDSN == "" {
		if dsn, ok := params["dsn"].(string); ok {
			spec.ExternalDSN = dsn
		}
	}
	return spec, DatabaseDerived{Plan: plan, EffectiveConfigs: effective}, nil
}

func versionKnown(kind catalog.Database, version string) bool {
	for _, v := range kind.Versions {
		if v.Version == version {
			return true
		}
	}
	return false
}

// CreateDatabase stores a definition (member+).
func (s *Service) CreateDatabase(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, w EntityWrite, spec DatabaseSpec) (Database, DatabaseDerived, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	e, err := header(actor, tenantID, w)
	if err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	spec, derived, err := s.DeriveDatabase(ctx, spec)
	if err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	d := Database{Entity: e, Spec: spec}
	if err := s.repo.InsertDatabase(ctx, d); err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "database.create", Target: audit.Target{Kind: "database", ID: d.ID.String(), Name: d.Name}}) //nolint:errcheck // audit never blocks
	return d, derived, nil
}

// GetDatabase reads one definition with its derived data (any member).
func (s *Service) GetDatabase(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Database, DatabaseDerived, []Usage, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Database{}, DatabaseDerived{}, nil, err
	}
	d, err := s.repo.DatabaseByID(ctx, id)
	if err != nil {
		return Database{}, DatabaseDerived{}, nil, err
	}
	if err := s.owned(tenantID, d.Entity, "database"); err != nil {
		return Database{}, DatabaseDerived{}, nil, err
	}
	_, derived, derr := s.DeriveDatabase(ctx, d.Spec)
	if derr != nil {
		derived.Validation = derr
	}
	usages, err := s.repo.TestsUsingDatabase(ctx, id)
	if err != nil {
		return Database{}, DatabaseDerived{}, nil, err
	}
	return d, derived, usages, nil
}

// ListDatabases lists definitions (any member).
func (s *Service) ListDatabases(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q ListQuery) ([]Database, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.Databases(ctx, tenantID, q)
}

// UpdateDatabase patches header and/or spec (member+).
func (s *Service) UpdateDatabase(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, p EntityPatch, specPatch *DatabaseSpec) (Database, DatabaseDerived, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	d, err := s.repo.DatabaseByID(ctx, id)
	if err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	if err := s.owned(tenantID, d.Entity, "database"); err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" || len(n) > 128 {
			return Database{}, DatabaseDerived{}, errs.Invalid("name must be 1..128 characters")
		}
		p.Name = &n
	}
	var toStore *DatabaseSpec
	if specPatch != nil {
		merged := d.Spec
		if specPatch.Version != "" {
			merged.Version = specPatch.Version
		}
		if specPatch.Image != "" {
			merged.Image = specPatch.Image
		}
		if specPatch.Params != nil {
			merged.Params = specPatch.Params
		}
		if specPatch.Configs != nil {
			merged.Configs = specPatch.Configs
		}
		merged, _, err = s.DeriveDatabase(ctx, merged)
		if err != nil {
			return Database{}, DatabaseDerived{}, err
		}
		toStore = &merged
	}
	if err := s.repo.UpdateDatabase(ctx, id, p, toStore); err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "database.update", Target: audit.Target{Kind: "database", ID: id.String(), Name: d.Name}}) //nolint:errcheck // audit never blocks
	d, derived, _, err := s.GetDatabase(ctx, actor, tenantID, id)
	return d, derived, err
}

// DeleteDatabase removes a definition; referenced ones only with inlineUsages.
func (s *Service) DeleteDatabase(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, inlineUsages bool) error {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return err
	}
	d, err := s.repo.DatabaseByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.owned(tenantID, d.Entity, "database"); err != nil {
		return err
	}
	usages, err := s.repo.TestsUsingDatabase(ctx, id)
	if err != nil {
		return err
	}
	if len(usages) > 0 {
		if !inlineUsages {
			return errs.Newf(errs.CodeConflict, "referenced by %d test(s); pass inline_usages to copy it into them", len(usages))
		}
		if err := s.repo.InlineDatabase(ctx, id, d.Spec); err != nil {
			return err
		}
	}
	if err := s.repo.DeleteDatabase(ctx, id); err != nil {
		return err
	}
	return s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "database.delete", Target: audit.Target{Kind: "database", ID: id.String(), Name: d.Name}})
}

// CloneDatabase copies a definition under a new name.
func (s *Service) CloneDatabase(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, name string) (Database, DatabaseDerived, error) {
	src, _, _, err := s.GetDatabase(ctx, actor, tenantID, id)
	if err != nil {
		return Database{}, DatabaseDerived{}, err
	}
	if name == "" {
		name = src.Name + " (copy)"
	}
	return s.CreateDatabase(ctx, actor, tenantID, EntityWrite{Name: name, Description: src.Description, Tags: src.Tags}, src.Spec)
}

// --- workloads --------------------------------------------------------------

// DeriveWorkload bakes the workload through workload.stroppy@1 and checks
// it against the stroppy catalog.
func (s *Service) DeriveWorkload(ctx context.Context, spec WorkloadSpec) (WorkloadSpec, json.RawMessage, WorkloadDerived, error) {
	ver, ok := s.catalog.StroppyVersion(spec.StroppyVersion)
	if !ok {
		return spec, nil, WorkloadDerived{}, errs.Invalid(fmt.Sprintf("unknown stroppy version %q", spec.StroppyVersion))
	}
	if !hasProtocol(ver.Protocols, spec.Protocol) {
		return spec, nil, WorkloadDerived{}, errs.Invalid(fmt.Sprintf("stroppy %s does not speak %s", ver.Version, spec.Protocol))
	}
	if len(spec.Segments) == 0 {
		return spec, nil, WorkloadDerived{}, errs.Invalid("at least one segment")
	}
	full := map[string]json.RawMessage{}
	if len(spec.Options) > 0 {
		if err := json.Unmarshal(spec.Options, &full); err != nil {
			return spec, nil, WorkloadDerived{}, errs.Invalid("options is not a JSON object")
		}
	}
	full["stroppy_version"], _ = json.Marshal(spec.StroppyVersion) //nolint:errcheck // string
	full["protocol"], _ = json.Marshal(spec.Protocol)              //nolint:errcheck // string
	full["segments"], _ = json.Marshal(spec.Segments)              //nolint:errcheck // raw list
	raw, _ := json.Marshal(full)                                   //nolint:errcheck // map of raw
	baked, err := s.validate.Bake(ctx, "workload.stroppy@1", raw)
	if err != nil {
		return spec, nil, WorkloadDerived{}, err
	}
	var value struct {
		Segments []map[string]any `json:"segments"`
	}
	_ = json.Unmarshal(baked, &value) //nolint:errcheck // baked by us
	derived := WorkloadDerived{}
	maxVUs, maxWorkers := 1, 1
	for i, seg := range value.Segments {
		sum := SegmentSummary{Name: str(seg["name"]), Steps: []string{}}
		wl, _ := seg["workload"].(map[string]any) //nolint:errcheck // baked shape
		sum.Script = str(wl["script"])
		if !ver.HasScript(sum.Script) {
			return spec, nil, WorkloadDerived{}, errs.Invalid(fmt.Sprintf("segments[%d]: stroppy %s has no script %q", i, ver.Version, sum.Script))
		}
		if sc, ok := ver.Script(sum.Script); ok && len(sc.Protocols) > 0 && !hasProtocol(sc.Protocols, spec.Protocol) {
			return spec, nil, WorkloadDerived{}, errs.Invalid(fmt.Sprintf("segments[%d]: script %s does not support %s", i, sum.Script, spec.Protocol))
		}
		if run, ok := seg["run"].(map[string]any); ok {
			sum.VUs = intOf(run["vus"])
			if d := str(run["duration"]); d != "" && d != "0s" {
				sum.Limit = d
			} else if it := intOf(run["iterations"]); it > 0 {
				sum.Limit = fmt.Sprintf("%d iterations", it)
			}
		}
		if lw := intOf(wl["load_workers"]); lw > maxWorkers {
			maxWorkers = lw
		}
		if sum.VUs > maxVUs {
			maxVUs = sum.VUs
		}
		if steps, ok := seg["steps"].([]any); ok {
			for _, st := range steps {
				sum.Steps = append(sum.Steps, str(st))
			}
		}
		derived.Segments = append(derived.Segments, sum)
	}
	derived.Runner = topology.RunnerRequirement(maxVUs, maxWorkers)
	// Segments come back resolved too.
	if segs, ok := full["segments"]; ok && len(segs) > 0 {
		var resolved struct {
			Segments []json.RawMessage `json:"segments"`
		}
		_ = json.Unmarshal(baked, &resolved) //nolint:errcheck // baked by us
		spec.Segments = resolved.Segments
	}
	return spec, baked, derived, nil
}

func hasProtocol(list []catalog.Protocol, p catalog.Protocol) bool {
	for _, x := range list {
		if x == p {
			return true
		}
	}
	return false
}

// CreateWorkload stores a definition (member+).
func (s *Service) CreateWorkload(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, w EntityWrite, spec WorkloadSpec) (Workload, WorkloadDerived, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	e, err := header(actor, tenantID, w)
	if err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	spec, baked, derived, err := s.DeriveWorkload(ctx, spec)
	if err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	wl := Workload{Entity: e, Spec: spec, Baked: baked}
	if err := s.repo.InsertWorkload(ctx, wl); err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "workload.create", Target: audit.Target{Kind: "workload", ID: wl.ID.String(), Name: wl.Name}}) //nolint:errcheck // audit never blocks
	return wl, derived, nil
}

// GetWorkload reads one definition (any member).
func (s *Service) GetWorkload(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Workload, WorkloadDerived, []Usage, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Workload{}, WorkloadDerived{}, nil, err
	}
	w, err := s.repo.WorkloadByID(ctx, id)
	if err != nil {
		return Workload{}, WorkloadDerived{}, nil, err
	}
	if err := s.owned(tenantID, w.Entity, "workload"); err != nil {
		return Workload{}, WorkloadDerived{}, nil, err
	}
	_, _, derived, derr := s.DeriveWorkload(ctx, w.Spec)
	if derr != nil {
		derived.Validation = derr
	}
	usages, err := s.repo.TestsUsingWorkload(ctx, id)
	if err != nil {
		return Workload{}, WorkloadDerived{}, nil, err
	}
	return w, derived, usages, nil
}

// ListWorkloads lists definitions (any member).
func (s *Service) ListWorkloads(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q ListQuery) ([]Workload, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.Workloads(ctx, tenantID, q)
}

// UpdateWorkload patches header and/or spec (member+).
func (s *Service) UpdateWorkload(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, p EntityPatch, specPatch *WorkloadSpec) (Workload, WorkloadDerived, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	w, err := s.repo.WorkloadByID(ctx, id)
	if err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	if err := s.owned(tenantID, w.Entity, "workload"); err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" || len(n) > 128 {
			return Workload{}, WorkloadDerived{}, errs.Invalid("name must be 1..128 characters")
		}
		p.Name = &n
	}
	var toStore *WorkloadSpec
	var baked json.RawMessage
	if specPatch != nil {
		merged := w.Spec
		if specPatch.StroppyVersion != "" {
			merged.StroppyVersion = specPatch.StroppyVersion
		}
		if specPatch.Protocol != "" {
			merged.Protocol = specPatch.Protocol
		}
		if specPatch.Segments != nil {
			merged.Segments = specPatch.Segments
		}
		if specPatch.Options != nil {
			merged.Options = specPatch.Options
		}
		merged, baked, _, err = s.DeriveWorkload(ctx, merged)
		if err != nil {
			return Workload{}, WorkloadDerived{}, err
		}
		toStore = &merged
	}
	if err := s.repo.UpdateWorkload(ctx, id, p, toStore, baked); err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "workload.update", Target: audit.Target{Kind: "workload", ID: id.String(), Name: w.Name}}) //nolint:errcheck // audit never blocks
	w, derived, _, err := s.GetWorkload(ctx, actor, tenantID, id)
	return w, derived, err
}

// DeleteWorkload removes a definition; referenced ones only with inlineUsages.
func (s *Service) DeleteWorkload(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, inlineUsages bool) error {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return err
	}
	w, err := s.repo.WorkloadByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.owned(tenantID, w.Entity, "workload"); err != nil {
		return err
	}
	usages, err := s.repo.TestsUsingWorkload(ctx, id)
	if err != nil {
		return err
	}
	if len(usages) > 0 {
		if !inlineUsages {
			return errs.Newf(errs.CodeConflict, "referenced by %d test(s); pass inline_usages to copy it into them", len(usages))
		}
		if err := s.repo.InlineWorkload(ctx, id, w.Spec); err != nil {
			return err
		}
	}
	if err := s.repo.DeleteWorkload(ctx, id); err != nil {
		return err
	}
	return s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "workload.delete", Target: audit.Target{Kind: "workload", ID: id.String(), Name: w.Name}})
}

// CloneWorkload copies a definition under a new name.
func (s *Service) CloneWorkload(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, name string) (Workload, WorkloadDerived, error) {
	src, _, _, err := s.GetWorkload(ctx, actor, tenantID, id)
	if err != nil {
		return Workload{}, WorkloadDerived{}, err
	}
	if name == "" {
		name = src.Name + " (copy)"
	}
	return s.CreateWorkload(ctx, actor, tenantID, EntityWrite{Name: name, Description: src.Description, Tags: src.Tags}, src.Spec)
}

func str(v any) string {
	s, _ := v.(string) //nolint:errcheck // absent = empty
	return s
}

func intOf(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}
