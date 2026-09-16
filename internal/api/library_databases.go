package api

import (
	"context"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func databaseSpecFrom(kind oas.DatabaseKind, version string, image oas.OptString, params oas.SchemaValue, configs map[string]map[string]oas.SchemaValue, dsn oas.OptString) library.DatabaseSpec {
	return library.DatabaseSpec{
		Kind: catalog.DatabaseKind(kind), Version: version, Image: image.Or(""), Params: rawOf(params),
		Configs: configsFrom(configs), ExternalDSN: dsn.Or(""),
	}
}

func databaseSpecOf(s oas.DatabaseSpec) library.DatabaseSpec {
	var configs map[string]map[string]oas.SchemaValue
	if c, ok := s.Configs.Get(); ok {
		configs = map[string]map[string]oas.SchemaValue{}
		for role, item := range c {
			configs[role] = item
		}
	}
	var dsn oas.OptString
	if ext, ok := s.External.Get(); ok {
		dsn = ext.Dsn
	}
	out := databaseSpecFrom(s.Kind, s.Version, s.Image, s.Params, configs, dsn)
	if v, ok := s.Runtime.Get(); ok {
		out.Runtime = rawOf(v)
	}
	return out
}

func databaseWriteSpec(w *oas.DatabaseWrite) library.DatabaseSpec {
	var configs map[string]map[string]oas.SchemaValue
	if c, ok := w.Configs.Get(); ok {
		configs = map[string]map[string]oas.SchemaValue{}
		for role, item := range c {
			configs[role] = item
		}
	}
	var dsn oas.OptString
	if ext, ok := w.External.Get(); ok {
		dsn = ext.Dsn
	}
	out := databaseSpecFrom(w.Kind, w.Version, w.Image, w.Params, configs, dsn)
	if v, ok := w.Runtime.Get(); ok {
		out.Runtime = rawOf(v)
	}
	return out
}

func specToWire(spec library.DatabaseSpec) oas.DatabaseSpec {
	out := oas.DatabaseSpec{Kind: oas.DatabaseKind(spec.Kind), Version: spec.Version, Params: schemaValueOf(spec.Params)}
	if len(spec.Runtime) > 0 {
		out.Runtime = oas.NewOptSchemaValue(schemaValueOf(spec.Runtime))
	}
	if spec.Image != "" {
		out.Image = oas.NewOptString(spec.Image)
	}
	if len(spec.Configs) > 0 {
		c := oas.DatabaseSpecConfigs{}
		for role, byID := range configsOf(spec.Configs) {
			c[role] = byID
		}
		out.Configs = oas.NewOptDatabaseSpecConfigs(c)
	}
	if kind, ok := schemaRefOf(spec.Kind); ok {
		out.Schema = oas.NewOptSchemaRef(kind)
	}
	return out
}

func schemaRefOf(kind catalog.DatabaseKind) (oas.SchemaRef, bool) {
	return oas.SchemaRef{ID: "db." + string(kind) + ".params", Version: "1"}, true
}

func (h *Handler) databaseOf(d library.Database, derived library.DatabaseDerived, usages []library.Usage) *oas.Database {
	hd := entityHeader(d.Entity)
	spec := specToWire(d.Spec)
	out := &oas.Database{
		ID: hd.id, Name: hd.name, Description: hd.description, Tags: oas.NewOptDatabaseTags(tagsOf(d.Tags)), Author: hd.author, CreatedAt: hd.created, UpdatedAt: hd.updated,
		Runtime: spec.Runtime, Kind: spec.Kind, Version: spec.Version, Image: spec.Image, Params: spec.Params, Schema: spec.Schema, Usages: usagesOf(usages),
	}
	if c, ok := spec.Configs.Get(); ok {
		dc := oas.DatabaseConfigs{}
		for role, byID := range c {
			dc[role] = oas.DatabaseConfigsItem(byID)
		}
		out.Configs = oas.NewOptDatabaseConfigs(dc)
	}
	if derived.Validation != nil {
		out.Validation = validationErrOf(derived.Validation)
		return out
	}
	out.TopologyPreview = oas.NewOptTopologyPreview(topologyOf(derived.Plan))
	out.Requirements = oas.NewOptRequirements(requirementsOf(derived.Plan.Requirements))
	eff := oas.DatabaseEffectiveConfigs{}
	for role, byID := range configsOf(derived.EffectiveConfigs) {
		eff[role] = byID
	}
	out.EffectiveConfigs = oas.NewOptDatabaseEffectiveConfigs(eff)
	out.Validation = oas.NewOptValidationResult(oas.ValidationResult{Errors: []oas.ValidationError{}})
	return out
}

func listQueryOf(search, tags, author oas.OptString, sort, order string, cursor oas.OptString, limit oas.OptInt) (library.ListQuery, int, error) {
	offset, err := cursorOffset(cursor)
	if err != nil {
		return library.ListQuery{}, 0, err
	}
	lim := limit.Or(50)
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	return library.ListQuery{
		Search: search.Or(""), Tags: tagsFilterOf(tags), AuthorID: author.Or(""), Sort: sort, Desc: order == "desc", Limit: lim + 1, Offset: offset,
	}, lim, nil
}

// ListDatabases — definitions of the tenant.
func (h *Handler) ListDatabases(ctx context.Context, params oas.ListDatabasesParams) (*oas.ListDatabasesOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	sortKey := ""
	if v, ok := params.Sort.Get(); ok {
		sortKey = string(v)
	}
	order := ""
	if v, ok := params.Order.Get(); ok {
		order = string(v)
	}
	q, limit, err := listQueryOf(params.Search, params.Tags, params.Author, sortKey, order, params.Cursor, params.Limit)
	if err != nil {
		return nil, err
	}
	for _, k := range params.Kind {
		q.Kinds = append(q.Kinds, string(k))
	}
	list, err := h.deps.Library.ListDatabases(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, q.Offset, limit)
	out := &oas.ListDatabasesOK{Data: make([]oas.Database, 0, len(list)), Meta: meta}
	for _, d := range list {
		_, derived, derr := h.deps.Library.DeriveDatabase(ctx, d.Spec)
		if derr != nil {
			derived.Validation = derr
		}
		out.Data = append(out.Data, *h.databaseOf(d, derived, nil))
	}
	return out, nil
}

// CreateDatabase — store a definition.
func (h *Handler) CreateDatabase(ctx context.Context, req *oas.DatabaseWrite, params oas.CreateDatabaseParams) (*oas.Database, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	w := library.EntityWrite{Name: req.Name, Description: req.Description.Or("")}
	if tags, ok := req.Tags.Get(); ok {
		w.Tags = tags
	}
	d, derived, err := h.deps.Library.CreateDatabase(ctx, a, t.ID, w, databaseWriteSpec(req))
	if err != nil {
		return nil, err
	}
	return h.databaseOf(d, derived, nil), nil
}

// GetDatabase — one definition with derived data and usages.
func (h *Handler) GetDatabase(ctx context.Context, params oas.GetDatabaseParams) (*oas.Database, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	d, derived, usages, err := h.deps.Library.GetDatabase(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.databaseOf(d, derived, usages), nil
}

// PatchDatabase — update header and/or spec.
func (h *Handler) PatchDatabase(ctx context.Context, req *oas.DatabasePatch, params oas.PatchDatabaseParams) (*oas.Database, error) {
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
	var spec *library.DatabaseSpec
	if req.Version.Set || req.Image.Set || req.Params.Set || req.Configs.Set || req.Runtime.Set {
		spec = &library.DatabaseSpec{Version: req.Version.Or(""), Image: req.Image.Or("")}
		if v, ok := req.Runtime.Get(); ok {
			spec.Runtime = rawOf(v)
		}
		if v, ok := req.Params.Get(); ok {
			spec.Params = rawOf(v)
		}
		if c, ok := req.Configs.Get(); ok {
			m := map[string]map[string]oas.SchemaValue{}
			for role, item := range c {
				m[role] = item
			}
			spec.Configs = configsFrom(m)
		}
	}
	d, derived, err := h.deps.Library.UpdateDatabase(ctx, a, t.ID, params.ID, p, spec)
	if err != nil {
		return nil, err
	}
	return h.databaseOf(d, derived, nil), nil
}

// DeleteDatabase — remove (inline into tests when asked).
func (h *Handler) DeleteDatabase(ctx context.Context, params oas.DeleteDatabaseParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Library.DeleteDatabase(ctx, a, t.ID, params.ID, params.InlineUsages.Or(false))
}

// PreviewDatabase — validate an unsaved definition.
func (h *Handler) PreviewDatabase(ctx context.Context, req *oas.DatabaseWrite, params oas.PreviewDatabaseParams) (*oas.DatabasePreview, error) {
	if _, _, err := h.tenantOf(ctx, params.Slug); err != nil {
		return nil, err
	}
	_, derived, err := h.deps.Library.DeriveDatabase(ctx, databaseWriteSpec(req))
	if err != nil {
		if e, ok := errs.AsValidation(err); ok {
			if vr, ok := e.Validation.(*schemapb.ValidationResult); ok {
				return &oas.DatabasePreview{Validation: validationOf(vr)}, nil
			}
		}
		return nil, err
	}
	eff := oas.DatabasePreviewEffectiveConfigs{}
	for role, byID := range configsOf(derived.EffectiveConfigs) {
		eff[role] = byID
	}
	return &oas.DatabasePreview{
		Validation:       oas.ValidationResult{Errors: []oas.ValidationError{}},
		TopologyPreview:  oas.NewOptTopologyPreview(topologyOf(derived.Plan)),
		Requirements:     oas.NewOptRequirements(requirementsOf(derived.Plan.Requirements)),
		EffectiveConfigs: oas.NewOptDatabasePreviewEffectiveConfigs(eff),
	}, nil
}

// CloneDatabase — copy.
func (h *Handler) CloneDatabase(ctx context.Context, req oas.OptCloneRequest, params oas.CloneDatabaseParams) (*oas.Database, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	name := ""
	if r, ok := req.Get(); ok {
		name = r.Name.Or("")
	}
	d, derived, err := h.deps.Library.CloneDatabase(ctx, a, t.ID, params.ID, name)
	if err != nil {
		return nil, err
	}
	return h.databaseOf(d, derived, nil), nil
}

// ExportDatabase — portable document (JSON; YAML when asked).
func (h *Handler) ExportDatabase(ctx context.Context, params oas.ExportDatabaseParams) (oas.ExportDatabaseRes, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	doc, err := h.deps.Library.ExportDatabase(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	if wantsYAML(ctx) {
		r, err := yamlDocument(doc)
		if err != nil {
			return nil, err
		}
		return &oas.ExportDatabaseOKApplicationYaml{Data: r}, nil
	}
	return exportDocumentOf(doc), nil
}

// ImportDatabase — create or update by name.
func (h *Handler) ImportDatabase(ctx context.Context, req oas.ImportDatabaseReq, params oas.ImportDatabaseParams) (*oas.Database, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	doc, err := documentOf(req)
	if err != nil {
		return nil, err
	}
	d, derived, _, err := h.deps.Library.ImportDatabase(ctx, a, t.ID, doc)
	if err != nil {
		return nil, err
	}
	return h.databaseOf(d, derived, nil), nil
}

// DiffDatabases — field-by-field diff.
func (h *Handler) DiffDatabases(ctx context.Context, params oas.DiffDatabasesParams) (*oas.Diff, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	changes, err := h.deps.Library.DiffDatabases(ctx, a, t.ID, params.A, params.B)
	if err != nil {
		return nil, err
	}
	return diffOf(changes), nil
}
