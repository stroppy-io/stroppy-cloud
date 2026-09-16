// Package schemas is the server's schemapb registry: every product schema
// from the pipelines module, compiled once, plus Bake for the "validate on
// every door" rule. Validation failures are domain errors carrying the
// schemapb result, so the API renders them as Problem.validation.
package schemas

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Registry holds compiled engines by public id (`ns.name@major`).
type Registry struct {
	byID    map[string]*schemapb.Schema
	mu      sync.Mutex
	engines map[string]*schemapb.Engine
}

// New indexes every product schema.
func New() *Registry {
	return &Registry{byID: schemas.ByID(), engines: map[string]*schemapb.Engine{}}
}

// IDs lists the registered ids.
func (r *Registry) IDs() []string {
	out := make([]string, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	return out
}

// Info describes a schema for the listing.
func (r *Registry) Info(id string) (catalog.SchemaInfo, bool) {
	s, ok := r.byID[id]
	if !ok {
		return catalog.SchemaInfo{}, false
	}
	info := catalog.SchemaInfo{
		ID: id, Namespace: s.GetId().GetNamespace(), Name: s.GetId().GetName(), Version: s.GetId().GetVersion(),
		Description: s.GetDescription(), Templates: []string{},
	}
	for name := range s.GetTemplates() {
		info.Templates = append(info.Templates, name)
	}
	sort.Strings(info.Templates)
	return info, true
}

// Render renders a value through a named template of the schema (config
// files). The value is baked first.
func (r *Registry) Render(_ context.Context, id, template string, value json.RawMessage) (string, error) {
	e, err := r.engine(id)
	if err != nil {
		return "", err
	}
	generic := map[string]any{}
	if len(value) > 0 {
		generic, err = spec.DecodeObject(value)
		if err != nil || generic == nil {
			return "", errs.Invalid("value is not a JSON object")
		}
	}
	baked, res, err := e.Bake(generic)
	if err != nil {
		return "", errs.Wrap(errs.CodeInvalid, "value cannot be baked", err)
	}
	if res.Blocking() {
		return "", &errs.Error{Code: errs.CodeValidation, Detail: "value does not fit " + id, Validation: res}
	}
	if template == "" {
		for name := range e.Schema().GetTemplates() {
			if template == "" || name < template {
				template = name
			}
		}
	}
	if template == "" {
		return "", errs.Invalid("schema has no templates")
	}
	out, err := baked.Render(schemapb.TemplateName(template))
	if err != nil {
		return "", errs.Wrap(errs.CodeInvalid, "render", err)
	}
	return out, nil
}

// Schema returns a schema by id.
func (r *Registry) Schema(id string) (*schemapb.Schema, bool) {
	s, ok := r.byID[id]
	return s, ok
}

func (r *Registry) engine(id string) (*schemapb.Engine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.engines[id]; ok {
		return e, nil
	}
	s, ok := r.byID[id]
	if !ok {
		return nil, errs.NotFound("schema " + id)
	}
	e, err := schemapb.Compile(s)
	if err != nil {
		return nil, fmt.Errorf("schema %s: compile: %w", id, err)
	}
	r.engines[id] = e
	return e, nil
}

// Bake validates value against the schema and returns the resolved value
// (defaults and computed fields applied). Blocking errors come back as a
// CodeValidation domain error with the schemapb result attached.
func (r *Registry) Bake(_ context.Context, id string, value json.RawMessage) (json.RawMessage, error) {
	e, err := r.engine(id)
	if err != nil {
		return nil, err
	}
	generic := map[string]any{}
	if len(value) > 0 {
		generic, err = spec.DecodeObject(value)
		if err != nil || generic == nil {
			return nil, errs.Invalid("value is not a JSON object")
		}
	}
	_, res, err := e.Bake(generic)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInvalid, "value cannot be baked", err)
	}
	if res.Blocking() {
		return nil, &errs.Error{Code: errs.CodeValidation, Detail: "value does not fit " + id, Validation: res}
	}
	out, err := json.Marshal(canonical(generic))
	if err != nil {
		return nil, err
	}
	return out, nil
}

// canonical renders resolved native values in their wire form: durations
// as "5m", timestamps as RFC 3339 — what the schema accepts back.
func canonical(v any) any {
	switch x := v.(type) {
	case time.Duration:
		return x.String()
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = canonical(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = canonical(val)
		}
		return out
	}
	return v
}

// Validate is Bake without the value: the result only.
func (r *Registry) Validate(_ context.Context, id string, value json.RawMessage) (*schemapb.ValidationResult, error) {
	e, err := r.engine(id)
	if err != nil {
		return nil, err
	}
	generic := map[string]any{}
	if len(value) > 0 {
		generic, err = spec.DecodeObject(value)
		if err != nil || generic == nil {
			return nil, errs.Invalid("value is not a JSON object")
		}
	}
	return e.Validate(generic), nil
}

// MaskSecrets replaces the values of the schema's secret fields (at any
// depth) with "***"; unknown ids mask nothing.
func (r *Registry) MaskSecrets(id string, value json.RawMessage) json.RawMessage {
	sch, ok := r.Schema(id)
	if !ok || len(value) == 0 {
		return value
	}
	v, err := spec.DecodeObject(value)
	if err != nil {
		return value
	}
	out, err := json.Marshal(maskFields(sch.GetFields(), v))
	if err != nil {
		return value
	}
	return out
}

func maskFields(fields []*schemapb.Schema_Field, v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	for _, f := range fields {
		x, present := m[f.GetName()]
		if !present {
			continue
		}
		m[f.GetName()] = maskField(f, x)
	}
	return m
}

func maskField(f *schemapb.Schema_Field, x any) any {
	if f.GetSecret() {
		return "***"
	}
	switch {
	case f.GetObject() != nil:
		return maskFields(f.GetObject().GetSchema().GetFields(), x)
	case f.GetList() != nil:
		items, ok := x.([]any)
		if !ok || len(f.GetList().GetItems()) == 0 {
			return x
		}
		item := f.GetList().GetItems()[0]
		for i := range items {
			items[i] = maskField(item, items[i])
		}
		return items
	case f.GetMap() != nil:
		m, ok := x.(map[string]any)
		if !ok {
			return x
		}
		for k := range m {
			m[k] = maskField(f.GetMap().GetValueField(), m[k])
		}
		return m
	}
	return x
}
