package library

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

/*
DOCUMENTS: the portable form of a definition — `apiVersion/kind/metadata/
spec` like a manifest, so a CI pipeline can `apply` it and a person can
read it. Import creates or updates by metadata.name.
*/

// APIVersion of the documents.
const APIVersion = "stroppy.io/v1"

// Document is the portable form.
type Document struct {
	APIVersion string          `json:"api_version"`
	Kind       string          `json:"kind"`
	Metadata   Metadata        `json:"metadata"`
	Spec       json.RawMessage `json:"spec"`
}

// Metadata of a document.
type Metadata struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

func document(kind string, e Entity, spec any) (Document, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return Document{}, err
	}
	return Document{APIVersion: APIVersion, Kind: kind, Metadata: Metadata{Name: e.Name, Description: e.Description, Tags: e.Tags}, Spec: raw}, nil
}

func (d Document) check(kind string) error {
	if d.APIVersion != "" && d.APIVersion != APIVersion {
		return errs.Invalid("api_version must be " + APIVersion)
	}
	if d.Kind != kind {
		return errs.Invalid("kind must be " + kind)
	}
	if strings.TrimSpace(d.Metadata.Name) == "" {
		return errs.Invalid("metadata.name is required")
	}
	if len(d.Spec) == 0 {
		return errs.Invalid("spec is required")
	}
	return nil
}

// ExportDatabase renders a database as a document.
func (s *Service) ExportDatabase(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Document, error) {
	d, _, _, err := s.GetDatabase(ctx, actor, tenantID, id)
	if err != nil {
		return Document{}, err
	}
	spec := d.Spec
	spec.ExternalDSN = "" // write-only
	return document("Database", d.Entity, spec)
}

// ImportDatabase creates or updates (by name) from a document.
func (s *Service) ImportDatabase(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, doc Document) (Database, DatabaseDerived, bool, error) {
	if err := doc.check("Database"); err != nil {
		return Database{}, DatabaseDerived{}, false, err
	}
	var spec DatabaseSpec
	if err := json.Unmarshal(doc.Spec, &spec); err != nil {
		return Database{}, DatabaseDerived{}, false, errs.Wrap(errs.CodeInvalid, "spec", err)
	}
	w := EntityWrite{Name: doc.Metadata.Name, Description: doc.Metadata.Description, Tags: doc.Metadata.Tags}
	existing, err := s.repo.Databases(ctx, tenantID, ListQuery{Search: w.Name, Limit: 50})
	if err != nil {
		return Database{}, DatabaseDerived{}, false, err
	}
	for _, e := range existing {
		if e.Name == w.Name {
			d, derived, err := s.UpdateDatabase(ctx, actor, tenantID, e.ID, EntityPatch{Description: &w.Description, Tags: w.Tags}, &spec)
			return d, derived, false, err
		}
	}
	d, derived, err := s.CreateDatabase(ctx, actor, tenantID, w, spec)
	return d, derived, true, err
}

// ExportWorkload renders a workload as a document.
func (s *Service) ExportWorkload(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Document, error) {
	w, _, _, err := s.GetWorkload(ctx, actor, tenantID, id)
	if err != nil {
		return Document{}, err
	}
	return document("Workload", w.Entity, w.Spec)
}

// ImportWorkload creates or updates (by name) from a document.
func (s *Service) ImportWorkload(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, doc Document) (Workload, WorkloadDerived, bool, error) {
	if err := doc.check("Workload"); err != nil {
		return Workload{}, WorkloadDerived{}, false, err
	}
	var spec WorkloadSpec
	if err := json.Unmarshal(doc.Spec, &spec); err != nil {
		return Workload{}, WorkloadDerived{}, false, errs.Wrap(errs.CodeInvalid, "spec", err)
	}
	w := EntityWrite{Name: doc.Metadata.Name, Description: doc.Metadata.Description, Tags: doc.Metadata.Tags}
	existing, err := s.repo.Workloads(ctx, tenantID, ListQuery{Search: w.Name, Limit: 50})
	if err != nil {
		return Workload{}, WorkloadDerived{}, false, err
	}
	for _, e := range existing {
		if e.Name == w.Name {
			wl, derived, err := s.UpdateWorkload(ctx, actor, tenantID, e.ID, EntityPatch{Description: &w.Description, Tags: w.Tags}, &spec)
			return wl, derived, false, err
		}
	}
	wl, derived, err := s.CreateWorkload(ctx, actor, tenantID, w, spec)
	return wl, derived, true, err
}

// ExportTest renders a test as a document. References are kept as ids
// with names alongside; inline specs travel whole.
func (s *Service) ExportTest(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Document, error) {
	t, _, res, err := s.GetTest(ctx, actor, tenantID, id)
	if err != nil {
		return Document{}, err
	}
	spec := map[string]any{}
	switch {
	case t.Spec.DatabaseRef != nil:
		ref := map[string]any{"id": t.Spec.DatabaseRef.String()}
		if res.Database != nil {
			ref["name"] = res.Database.Name
		}
		spec["database"] = map[string]any{"ref": ref}
	case t.Spec.DatabaseInline != nil:
		inline := *t.Spec.DatabaseInline
		inline.ExternalDSN = ""
		spec["database"] = map[string]any{"inline": inline}
	}
	switch {
	case t.Spec.WorkloadRef != nil:
		ref := map[string]any{"id": t.Spec.WorkloadRef.String()}
		if res.Workload != nil {
			ref["name"] = res.Workload.Name
		}
		spec["workload"] = map[string]any{"ref": ref}
	case t.Spec.WorkloadInline != nil:
		spec["workload"] = map[string]any{"inline": *t.Spec.WorkloadInline}
	}
	spec["sizes"] = t.Spec.Sizes
	if t.Spec.ProviderProfileID != nil {
		spec["provider_profile_id"] = t.Spec.ProviderProfileID.String()
	}
	if t.Spec.Keep > 0 {
		spec["keep"] = t.Spec.Keep.String()
	}
	spec["rating"] = map[string]bool{"tenant": t.Spec.RatingTenant, "global": t.Spec.RatingGlobal}
	return document("Test", t.Entity, spec)
}

// ImportTest creates or updates (by name) from a document. References by
// name resolve inside the tenant.
func (s *Service) ImportTest(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, doc Document) (Test, Fit, Resolved, bool, error) {
	if err := doc.check("Test"); err != nil {
		return Test{}, Fit{}, Resolved{}, false, err
	}
	spec, err := s.testSpecFromJSON(ctx, tenantID, doc.Spec)
	if err != nil {
		return Test{}, Fit{}, Resolved{}, false, err
	}
	w := EntityWrite{Name: doc.Metadata.Name, Description: doc.Metadata.Description, Tags: doc.Metadata.Tags}
	existing, err := s.repo.Tests(ctx, tenantID, ListQuery{Search: w.Name, Limit: 50})
	if err != nil {
		return Test{}, Fit{}, Resolved{}, false, err
	}
	for _, e := range existing {
		if e.Name == w.Name {
			t, fit, res, err := s.UpdateTest(ctx, actor, tenantID, e.ID, TestPatch{
				EntityPatch: EntityPatch{Description: &w.Description, Tags: w.Tags},
				SetDatabase: true, DatabaseRef: spec.DatabaseRef, DatabaseInline: spec.DatabaseInline,
				SetWorkload: true, WorkloadRef: spec.WorkloadRef, WorkloadInline: spec.WorkloadInline,
				Sizes: spec.Sizes, Execution: spec.Execution, SetProvider: true, ProviderProfileID: spec.ProviderProfileID,
				Keep: &spec.Keep, RatingTenant: &spec.RatingTenant, RatingGlobal: &spec.RatingGlobal,
			})
			return t, fit, res, false, err
		}
	}
	t, fit, res, err := s.CreateTest(ctx, actor, tenantID, w, spec)
	return t, fit, res, true, err
}

// TestSpecFromDocument decodes a Test document's spec (examples quick-run).
func (s *Service) TestSpecFromDocument(ctx context.Context, tenantID uuid.UUID, doc Document) (TestSpec, error) {
	if err := doc.check("Test"); err != nil {
		return TestSpec{}, err
	}
	return s.testSpecFromJSON(ctx, tenantID, doc.Spec)
}

// testSpecFromJSON decodes the document form of a test spec.
func (s *Service) testSpecFromJSON(ctx context.Context, tenantID uuid.UUID, raw json.RawMessage) (TestSpec, error) {
	var doc struct {
		Database *struct {
			Ref    *struct{ ID, Name string } `json:"ref"`
			Inline *DatabaseSpec              `json:"inline"`
		} `json:"database"`
		Workload *struct {
			Ref    *struct{ ID, Name string } `json:"ref"`
			Inline *WorkloadSpec              `json:"inline"`
		} `json:"workload"`
		Sizes             map[string]RoleSize `json:"sizes"`
		ProviderProfileID string              `json:"provider_profile_id"`
		Keep              string              `json:"keep"`
		Rating            struct {
			Tenant *bool `json:"tenant"`
			Global *bool `json:"global"`
		} `json:"rating"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return TestSpec{}, errs.Wrap(errs.CodeInvalid, "spec", err)
	}
	spec := TestSpec{Sizes: doc.Sizes, RatingTenant: true}
	if doc.Database != nil {
		switch {
		case doc.Database.Inline != nil:
			spec.DatabaseInline = doc.Database.Inline
		case doc.Database.Ref != nil:
			id, err := s.resolveRef(ctx, tenantID, "database", doc.Database.Ref.ID, doc.Database.Ref.Name)
			if err != nil {
				return TestSpec{}, err
			}
			spec.DatabaseRef = &id
		}
	}
	if doc.Workload != nil {
		switch {
		case doc.Workload.Inline != nil:
			spec.WorkloadInline = doc.Workload.Inline
		case doc.Workload.Ref != nil:
			id, err := s.resolveRef(ctx, tenantID, "workload", doc.Workload.Ref.ID, doc.Workload.Ref.Name)
			if err != nil {
				return TestSpec{}, err
			}
			spec.WorkloadRef = &id
		}
	}
	if doc.ProviderProfileID != "" {
		id, err := uuid.Parse(doc.ProviderProfileID)
		if err != nil {
			return TestSpec{}, errs.Invalid("provider_profile_id is not a uuid")
		}
		spec.ProviderProfileID = &id
	}
	if doc.Keep != "" {
		d, err := parseDuration(doc.Keep)
		if err != nil {
			return TestSpec{}, errs.Invalid("keep is not a duration")
		}
		spec.Keep = d
	}
	if doc.Rating.Tenant != nil {
		spec.RatingTenant = *doc.Rating.Tenant
	}
	if doc.Rating.Global != nil {
		spec.RatingGlobal = *doc.Rating.Global
	}
	return spec, nil
}

// resolveRef finds a definition by id, else by name inside the tenant.
func (s *Service) resolveRef(ctx context.Context, tenantID uuid.UUID, kind, id, name string) (uuid.UUID, error) {
	if id != "" {
		u, err := uuid.Parse(id)
		if err != nil {
			return uuid.Nil, errs.Invalid(kind + ".ref.id is not a uuid")
		}
		return u, nil
	}
	if name == "" {
		return uuid.Nil, errs.Invalid(kind + ".ref needs id or name")
	}
	switch kind {
	case "database":
		list, err := s.repo.Databases(ctx, tenantID, ListQuery{Search: name, Limit: 50})
		if err != nil {
			return uuid.Nil, err
		}
		for _, d := range list {
			if d.Name == name {
				return d.ID, nil
			}
		}
	case "workload":
		list, err := s.repo.Workloads(ctx, tenantID, ListQuery{Search: name, Limit: 50})
		if err != nil {
			return uuid.Nil, err
		}
		for _, w := range list {
			if w.Name == name {
				return w.ID, nil
			}
		}
	}
	return uuid.Nil, errs.NotFound(fmt.Sprintf("%s %q", kind, name))
}

// --- diff -----------------------------------------------------------------

// Change is one difference between two documents.
type Change struct {
	Path string
	Op   string // add | remove | replace
	A, B json.RawMessage
}

// Diff compares two JSON values field by field.
func Diff(a, b json.RawMessage) ([]Change, error) {
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return nil, err
	}
	changes := []Change{}
	diffInto("", va, vb, &changes)
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}

func diffInto(path string, a, b any, out *[]Change) {
	ma, aok := a.(map[string]any)
	mb, bok := b.(map[string]any)
	if aok && bok {
		keys := map[string]bool{}
		for k := range ma {
			keys[k] = true
		}
		for k := range mb {
			keys[k] = true
		}
		for k := range keys {
			p := k
			if path != "" {
				p = path + "." + k
			}
			av, ain := ma[k]
			bv, bin := mb[k]
			switch {
			case ain && !bin:
				*out = append(*out, Change{Path: p, Op: "remove", A: raw(av)})
			case !ain && bin:
				*out = append(*out, Change{Path: p, Op: "add", B: raw(bv)})
			default:
				diffInto(p, av, bv, out)
			}
		}
		return
	}
	if string(raw(a)) != string(raw(b)) {
		*out = append(*out, Change{Path: path, Op: "replace", A: raw(a), B: raw(b)})
	}
}

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v) //nolint:errcheck // decoded JSON re-encodes
	return b
}

// DiffDatabases / DiffWorkloads / DiffTests compare two definitions.
func (s *Service) DiffDatabases(ctx context.Context, actor auth.Actor, tenantID, a, b uuid.UUID) ([]Change, error) {
	da, err := s.ExportDatabase(ctx, actor, tenantID, a)
	if err != nil {
		return nil, err
	}
	db, err := s.ExportDatabase(ctx, actor, tenantID, b)
	if err != nil {
		return nil, err
	}
	return Diff(da.Spec, db.Spec)
}

func (s *Service) DiffWorkloads(ctx context.Context, actor auth.Actor, tenantID, a, b uuid.UUID) ([]Change, error) {
	da, err := s.ExportWorkload(ctx, actor, tenantID, a)
	if err != nil {
		return nil, err
	}
	db, err := s.ExportWorkload(ctx, actor, tenantID, b)
	if err != nil {
		return nil, err
	}
	return Diff(da.Spec, db.Spec)
}

func (s *Service) DiffTests(ctx context.Context, actor auth.Actor, tenantID, a, b uuid.UUID) ([]Change, error) {
	da, err := s.ExportTest(ctx, actor, tenantID, a)
	if err != nil {
		return nil, err
	}
	db, err := s.ExportTest(ctx, actor, tenantID, b)
	if err != nil {
		return nil, err
	}
	return Diff(da.Spec, db.Spec)
}
