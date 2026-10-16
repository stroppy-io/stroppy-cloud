// Package catalog is the read-only reference data the forms feed on: which
// databases exist in which versions with which roles and topology
// templates, what the providers offer and how sizes map to machines, which
// stroppy builds and scripts are known, and which metrics a run reports.
//
// v0 is static Go data. Every table that has a schemapb schema is baked
// through it at startup (a mistake here fails the process, not a run).
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Validator bakes a value through a schema id.
type Validator interface {
	Bake(ctx context.Context, schemaID string, value json.RawMessage) (json.RawMessage, error)
}

// SchemaInfo is one registered schema in the listing.
type SchemaInfo struct {
	ID          string
	Namespace   string
	Name        string
	Version     string
	Description string
	Templates   []string
}

// Schemas lists and reads schemas.
type Schemas interface {
	IDs() []string
	Info(id string) (SchemaInfo, bool)
}

// Catalog is the assembled reference data.
type Catalog struct {
	Databases []Database
	Providers []Provider
	Stroppy   StroppyCatalog
	Metrics   []Metric
	schemas   Schemas
}

// New assembles the static catalog over the database matrix (dbkind owns
// it) and checks the schema-backed tables.
func New(ctx context.Context, v Validator, schemas Schemas, databases []Database) (*Catalog, error) {
	c := &Catalog{Databases: databases, Providers: providers(), Stroppy: stroppy(), Metrics: metrics(), schemas: schemas}
	if err := c.check(ctx, v); err != nil {
		return nil, err
	}
	return c, nil
}

// check bakes the size table and the stroppy catalog through their
// schemas and verifies every referenced schema id exists.
func (c *Catalog) check(ctx context.Context, v Validator) error {
	sizes := map[string]map[string]map[string]SizeSpec{}
	for _, p := range c.Providers {
		sizes[string(p.Kind)] = p.Sizes
	}
	if err := bakeAs(ctx, v, "system.sizes@1", sizes); err != nil {
		return fmt.Errorf("catalog: size table: %w", err)
	}
	if err := bakeAs(ctx, v, "system.stroppy_catalog@1", c.Stroppy); err != nil {
		return fmt.Errorf("catalog: stroppy: %w", err)
	}
	known := map[string]bool{}
	for _, id := range c.schemas.IDs() {
		known[id] = true
	}
	for _, d := range c.Databases {
		if !known[d.ParamsSchema] {
			return fmt.Errorf("catalog: %s: params schema %s not registered", d.Kind, d.ParamsSchema)
		}
		for _, r := range d.Roles {
			for _, s := range r.ConfigSchemas {
				if !known[s] {
					return fmt.Errorf("catalog: %s/%s: config schema %s not registered", d.Kind, r.Role, s)
				}
				if err := bakeAs(ctx, v, s, r.Seed(s)); err != nil {
					return fmt.Errorf("catalog: %s/%s: %s defaults: %w", d.Kind, r.Role, s, err)
				}
			}
		}
		for _, t := range d.Topologies {
			raw, _ := json.Marshal(t.Params) //nolint:errcheck // map always marshals
			if _, err := v.Bake(ctx, d.ParamsSchema, raw); err != nil {
				return fmt.Errorf("catalog: %s: topology %s: %w", d.Kind, t.ID, err)
			}
		}
	}
	for _, p := range c.Providers {
		if !known[p.SettingsSchema] || !known[p.CredentialsSchema] {
			return fmt.Errorf("catalog: %s: provider schemas not registered", p.Kind)
		}
	}
	return nil
}

// Seed is the recipe's values for one config schema (never nil).
func (r Role) Seed(schemaID string) map[string]any {
	out := map[string]any{}
	for k, v := range r.ConfigSeeds[schemaID] {
		out[k] = v
	}
	return out
}

func bakeAs(ctx context.Context, v Validator, id string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = v.Bake(ctx, id, raw)
	return err
}

// Database returns one kind.
func (c *Catalog) Database(kind DatabaseKind) (Database, bool) {
	for _, d := range c.Databases {
		if d.Kind == kind {
			return d, true
		}
	}
	return Database{}, false
}

// Provider returns one provider.
func (c *Catalog) Provider(kind string) (Provider, bool) {
	for _, p := range c.Providers {
		if string(p.Kind) == kind {
			return p, true
		}
	}
	return Provider{}, false
}

// StroppyVersion returns one build ("" = the default).
func (c *Catalog) StroppyVersion(version string) (StroppyVersion, bool) {
	for _, v := range c.Stroppy.Versions {
		if v.Version == version || (version == "" && v.Default) {
			return v, true
		}
	}
	return StroppyVersion{}, false
}

// SchemaList is the schema listing, optionally by namespace.
func (c *Catalog) SchemaList(namespace string) []SchemaInfo {
	ids := c.schemas.IDs()
	sort.Strings(ids)
	out := make([]SchemaInfo, 0, len(ids))
	for _, id := range ids {
		info, ok := c.schemas.Info(id)
		if !ok || (namespace != "" && !strings.HasPrefix(id, namespace+".")) {
			continue
		}
		out = append(out, info)
	}
	return out
}

// MetricsFor filters metrics by database kind ("" = all).
// Metrics is the whole metric catalog (the contract's MetricKey and
// MetricGroup are its keys and groups).
func Metrics() []Metric { return metrics() }

func (c *Catalog) MetricsFor(kind DatabaseKind) []Metric {
	if kind == "" {
		return c.Metrics
	}
	out := make([]Metric, 0, len(c.Metrics))
	for _, m := range c.Metrics {
		if len(m.DBKinds) == 0 {
			out = append(out, m)
			continue
		}
		for _, k := range m.DBKinds {
			if k == kind {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// HasScript reports a script of the build.
func (v StroppyVersion) HasScript(id string) bool {
	_, ok := v.Script(id)
	return ok
}

// Script returns a script of the build.
func (v StroppyVersion) Script(id string) (StroppyScript, bool) {
	for _, s := range v.Scripts {
		if s.ID == id {
			return s, true
		}
	}
	return StroppyScript{}, false
}
