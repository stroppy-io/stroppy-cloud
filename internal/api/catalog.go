package api

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/go-faster/jx"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// GetPublicCatalog exposes the same catalog identities as the authenticated API.
func (h *Handler) GetPublicCatalog(_ context.Context) (*oas.PublicCatalog, error) {
	out := &oas.PublicCatalog{Databases: []oas.PublicCatalogDatabasesItem{}, Providers: []oas.ProviderKind{}}
	for _, d := range h.deps.Catalog.Databases {
		item := oas.PublicCatalogDatabasesItem{Kind: oas.DatabaseKind(d.Kind), Title: oas.NewOptString(d.Title), Versions: []string{}, Topologies: []string{}}
		for _, v := range d.Versions {
			item.Versions = append(item.Versions, v.Version)
		}
		for _, topology := range d.Topologies {
			item.Topologies = append(item.Topologies, topology.ID)
		}
		out.Databases = append(out.Databases, item)
	}
	for _, p := range h.deps.Catalog.Providers {
		out.Providers = append(out.Providers, oas.ProviderKind(p.Kind))
	}
	return out, nil
}

// GetCatalogDatabases — kinds, versions, roles, topology templates.
func (h *Handler) GetCatalogDatabases(ctx context.Context) (*oas.GetCatalogDatabasesOK, error) {
	if _, err := actor(ctx); err != nil {
		return nil, err
	}
	out := &oas.GetCatalogDatabasesOK{Data: make([]oas.CatalogDatabase, 0, len(h.deps.Catalog.Databases))}
	for _, d := range h.deps.Catalog.Databases {
		out.Data = append(out.Data, databaseOf(d))
	}
	return out, nil
}

// GetCatalogProviders — locations, platforms, disks, sizes.
func (h *Handler) GetCatalogProviders(ctx context.Context) (*oas.GetCatalogProvidersOK, error) {
	if _, err := actor(ctx); err != nil {
		return nil, err
	}
	out := &oas.GetCatalogProvidersOK{Data: make([]oas.CatalogProvider, 0, len(h.deps.Catalog.Providers))}
	for _, p := range h.deps.Catalog.Providers {
		out.Data = append(out.Data, providerOf(p))
	}
	return out, nil
}

// GetCatalogStroppy — builds and scripts (one version when asked).
func (h *Handler) GetCatalogStroppy(ctx context.Context, params oas.GetCatalogStroppyParams) (*oas.StroppyCatalog, error) {
	if _, err := actor(ctx); err != nil {
		return nil, err
	}
	c := h.deps.Catalog.Stroppy
	out := &oas.StroppyCatalog{Source: oas.NewOptStroppyCatalogSource(oas.StroppyCatalogSource(c.Source)), Versions: []oas.StroppyCatalogVersionsItem{}}
	if v, ok := params.Version.Get(); ok && v != "" {
		ver, found := h.deps.Catalog.StroppyVersion(v)
		if !found {
			return nil, errs.NotFound("stroppy version " + v)
		}
		out.Versions = append(out.Versions, stroppyVersionOf(ver))
		return out, nil
	}
	for _, ver := range c.Versions {
		out.Versions = append(out.Versions, stroppyVersionOf(ver))
	}
	return out, nil
}

// GetCatalogMetrics — metric catalog, optionally per db kind.
func (h *Handler) GetCatalogMetrics(ctx context.Context, params oas.GetCatalogMetricsParams) (*oas.GetCatalogMetricsOK, error) {
	if _, err := actor(ctx); err != nil {
		return nil, err
	}
	kind := catalog.DatabaseKind("")
	if v, ok := params.DbKind.Get(); ok {
		kind = catalog.DatabaseKind(v)
	}
	ms := h.deps.Catalog.MetricsFor(kind)
	out := &oas.GetCatalogMetricsOK{Data: make([]oas.MetricDef, 0, len(ms))}
	for _, m := range ms {
		out.Data = append(out.Data, metricOf(m))
	}
	return out, nil
}

// ListSchemas — registered schemapb schemas.
func (h *Handler) ListSchemas(ctx context.Context, params oas.ListSchemasParams) (*oas.ListSchemasOK, error) {
	if _, err := actor(ctx); err != nil {
		return nil, err
	}
	infos := h.deps.Catalog.SchemaList(params.Namespace.Or(""))
	out := &oas.ListSchemasOK{Data: make([]oas.SchemaInfo, 0, len(infos))}
	for _, s := range infos {
		item := oas.SchemaInfo{ID: s.ID, Version: s.Version, Namespace: s.Namespace, Name: s.Name, Templates: s.Templates}
		if s.Description != "" {
			item.Description = oas.NewOptString(s.Description)
		}
		out.Data = append(out.Data, item)
	}
	return out, nil
}

// GetSchema — the schema in protoJSON.
func (h *Handler) GetSchema(ctx context.Context, params oas.GetSchemaParams) (oas.GetSchemaOK, error) {
	if _, err := actor(ctx); err != nil {
		return nil, err
	}
	s, ok := h.deps.Schemas.Schema(params.SchemaId)
	if !ok {
		return nil, errs.NotFound("schema " + params.SchemaId)
	}
	raw, err := protojson.Marshal(s)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	out := oas.GetSchemaOK{}
	for k, v := range m {
		out[k] = jx.Raw(v)
	}
	return out, nil
}

// ValidateSchemaValue — resolve + validate; the resolved value when it fits.
func (h *Handler) ValidateSchemaValue(ctx context.Context, req *oas.ValidateSchemaValueReq, params oas.ValidateSchemaValueParams) (*oas.ValidateSchemaValueOK, error) {
	if _, err := actor(ctx); err != nil {
		return nil, err
	}
	res, err := h.deps.Schemas.Validate(ctx, params.SchemaId, rawOf(req.Value))
	if err != nil {
		return nil, err
	}
	out := &oas.ValidateSchemaValueOK{Result: validationOf(res)}
	if !res.Blocking() {
		resolved, err := h.deps.Schemas.Bake(ctx, params.SchemaId, rawOf(req.Value))
		if err == nil {
			out.Resolved = oas.NewOptSchemaValue(schemaValueOf(resolved))
		}
	}
	return out, nil
}

// RenderSchemaValue — the file a config value renders to.
func (h *Handler) RenderSchemaValue(ctx context.Context, req *oas.RenderSchemaValueReq, params oas.RenderSchemaValueParams) (oas.RenderSchemaValueOK, error) {
	if _, err := actor(ctx); err != nil {
		return oas.RenderSchemaValueOK{}, err
	}
	text, err := h.deps.Schemas.Render(ctx, params.SchemaId, req.Template.Or(""), rawOf(req.Value))
	if err != nil {
		return oas.RenderSchemaValueOK{}, err
	}
	return oas.RenderSchemaValueOK{Data: strings.NewReader(text)}, nil
}

var _ io.Reader = (*strings.Reader)(nil)

func databaseOf(d catalog.Database) oas.CatalogDatabase {
	out := oas.CatalogDatabase{
		Kind: oas.DatabaseKind(d.Kind), Title: d.Title, ParamsSchema: d.ParamsSchema, Deployable: d.Deployable,
		Versions: make([]oas.CatalogDatabaseVersionsItem, 0, len(d.Versions)), Roles: make([]oas.CatalogDatabaseRolesItem, 0, len(d.Roles)),
		Topologies: make([]oas.CatalogDatabaseTopologiesItem, 0, len(d.Topologies)), Protocols: make([]oas.Protocol, 0, len(d.Protocols)),
	}
	if d.Description != "" {
		out.Description = oas.NewOptString(d.Description)
	}
	for _, v := range d.Versions {
		item := oas.CatalogDatabaseVersionsItem{Version: v.Version}
		if v.Image != "" {
			item.Image = oas.NewOptString(v.Image)
		}
		if v.Default {
			item.Default = oas.NewOptBool(true)
		}
		if v.Deprecated {
			item.Deprecated = oas.NewOptBool(true)
		}
		out.Versions = append(out.Versions, item)
	}
	for _, r := range d.Roles {
		item := oas.CatalogDatabaseRolesItem{Role: r.Role, Title: r.Title, ConfigSchemas: r.ConfigSchemas}
		if item.ConfigSchemas == nil {
			item.ConfigSchemas = []string{}
		}
		if r.Engine != "" {
			item.Engine = oas.NewOptString(r.Engine)
		}
		out.Roles = append(out.Roles, item)
	}
	for _, t := range d.Topologies {
		raw, _ := json.Marshal(t.Params) //nolint:errcheck // map always marshals
		item := oas.CatalogDatabaseTopologiesItem{ID: t.ID, Title: t.Title, Params: schemaValueOf(raw)}
		if t.Description != "" {
			item.Description = oas.NewOptString(t.Description)
		}
		out.Topologies = append(out.Topologies, item)
	}
	for _, p := range d.Protocols {
		out.Protocols = append(out.Protocols, oas.Protocol(p))
	}
	return out
}

func providerOf(p catalog.Provider) oas.CatalogProvider {
	out := oas.CatalogProvider{
		Kind: oas.ProviderKind(p.Kind), Title: p.Title,
		SettingsSchema: oas.NewOptString(p.SettingsSchema), CredentialsSchema: oas.NewOptString(p.CredentialsSchema),
		Locations: make([]oas.CatalogProviderLocationsItem, 0, len(p.Locations)), Platforms: make([]oas.CatalogProviderPlatformsItem, 0, len(p.Platforms)),
		DiskTypes: make([]oas.CatalogProviderDiskTypesItem, 0, len(p.DiskTypes)), Sizes: oas.CatalogProviderSizes{}, Images: make([]oas.CatalogProviderImagesItem, 0, len(p.Images)),
	}
	for _, l := range p.Locations {
		out.Locations = append(out.Locations, oas.CatalogProviderLocationsItem{ID: l.ID, Title: oas.NewOptString(l.Title)})
	}
	for _, l := range p.Platforms {
		out.Platforms = append(out.Platforms, oas.CatalogProviderPlatformsItem{ID: l.ID, Title: oas.NewOptString(l.Title)})
	}
	for _, d := range p.DiskTypes {
		out.DiskTypes = append(out.DiskTypes, oas.CatalogProviderDiskTypesItem{ID: d.ID, Title: oas.NewOptString(d.Title), MinGB: oas.NewOptInt(d.MinGB), StepGB: oas.NewOptInt(d.StepGB)})
	}
	for family, table := range p.Sizes {
		rows := make([]oas.SizeSpec, 0, len(catalog.Sizes))
		for _, size := range catalog.Sizes {
			spec, ok := table[size]
			if !ok {
				continue
			}
			rows = append(rows, oas.SizeSpec{Size: oas.Size(size), CPU: spec.CPU, MemoryGB: spec.MemoryGB, InstanceType: spec.InstanceType, DefaultDiskGB: oas.NewOptInt(spec.DefaultDiskGB)})
		}
		out.Sizes[family] = rows
	}
	for _, i := range p.Images {
		out.Images = append(out.Images, oas.CatalogProviderImagesItem{ID: i.ID, Os: i.OS})
	}
	return out
}

func stroppyVersionOf(v catalog.StroppyVersion) oas.StroppyCatalogVersionsItem {
	out := oas.StroppyCatalogVersionsItem{
		Version: v.Version, Image: v.Image, Default: oas.NewOptBool(v.Default), Deprecated: oas.NewOptBool(v.Deprecated), Baseline: oas.NewOptBool(v.Baseline),
		Protocols: make([]oas.Protocol, 0, len(v.Protocols)), Scripts: make([]oas.StroppyScript, 0, len(v.Scripts)),
	}
	for _, p := range v.Protocols {
		out.Protocols = append(out.Protocols, oas.Protocol(p))
	}
	for _, s := range v.Scripts {
		script := oas.StroppyScript{ID: s.ID, Title: s.Title, Protocols: []oas.Protocol{}, Steps: []oas.StroppyScriptStepsItem{}, Params: []oas.StroppyParam{}}
		if s.Description != "" {
			script.Description = oas.NewOptString(s.Description)
		}
		for _, p := range s.Protocols {
			script.Protocols = append(script.Protocols, oas.Protocol(p))
		}
		for _, st := range s.Steps {
			item := oas.StroppyScriptStepsItem{ID: st.ID}
			if st.Title != "" {
				item.Title = oas.NewOptString(st.Title)
			}
			if st.Description != "" {
				item.Description = oas.NewOptString(st.Description)
			}
			if st.Phase != "" {
				item.Phase = oas.NewOptStroppyScriptStepsItemPhase(oas.StroppyScriptStepsItemPhase(st.Phase))
			}
			script.Steps = append(script.Steps, item)
		}
		for _, p := range s.Params {
			item := oas.StroppyParam{Name: p.Name, Config: p.Config, Type: oas.StroppyParamType(p.Type)}
			if p.Scope != "" {
				item.Scope = oas.NewOptStroppyParamScope(oas.StroppyParamScope(p.Scope))
			}
			if p.Description != "" {
				item.Description = oas.NewOptString(p.Description)
			}
			if p.Default != nil {
				if raw, err := json.Marshal(p.Default); err == nil {
					item.Default = jx.Raw(raw)
				}
			}
			if p.DefaultDescription != "" {
				item.DefaultDescription = oas.NewOptString(p.DefaultDescription)
			}
			if p.Env != "" {
				item.Env = oas.NewOptString(p.Env)
			}
			script.Params = append(script.Params, item)
		}
		out.Scripts = append(out.Scripts, script)
	}
	return out
}

func metricOf(m catalog.Metric) oas.MetricDef {
	out := oas.MetricDef{
		Key: m.Key, Title: m.Title, Unit: m.Unit, HigherIsBetter: m.HigherIsBetter, Group: m.Group,
		Scope: oas.MetricDefScope(m.Scope), DbKinds: make([]oas.DatabaseKind, 0, len(m.DBKinds)), RatingEligible: oas.NewOptBool(m.RatingEligible),
	}
	if m.Description != "" {
		out.Description = oas.NewOptString(m.Description)
	}
	for _, k := range m.DBKinds {
		out.DbKinds = append(out.DbKinds, oas.DatabaseKind(k))
	}
	return out
}
