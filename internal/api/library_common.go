package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-faster/jx"
	"github.com/google/uuid"
	schemapb "github.com/gopherex/schemapb/go/schemapb"
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

/*
LIBRARY WIRE HELPERS: the entity header, derived blocks (topology preview,
requirements, effective configs), documents in JSON or YAML, cursors.
Listing cursors are plain offsets: library sizes are small and every sort
key is served by one query.
*/

// header is the wire form of an entity header.
type header struct {
	id          uuid.UUID
	name        string
	description oas.OptString
	author      oas.UserRef
	created     time.Time
	updated     time.Time
}

func entityHeader(e library.Entity) header {
	h := header{id: e.ID, name: e.Name, created: e.CreatedAt, updated: e.UpdatedAt}
	if e.AuthorID != nil {
		h.author.ID = e.AuthorID.String()
	}
	if e.Description != "" {
		h.description = oas.NewOptString(e.Description)
	}
	return h
}

func cursorOffset(cursor oas.OptString) (int, error) {
	c, ok := cursor.Get()
	if !ok || c == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(c)
	if err != nil || n < 0 {
		return 0, errs.Invalid("cursor")
	}
	return n, nil
}

// page trims a limit+1 slice and renders the meta.
func page[T any](items []T, offset, limit int) ([]T, oas.PageMeta) {
	meta := oas.PageMeta{HasMore: oas.NewOptBool(false)}
	if len(items) > limit {
		items = items[:limit]
		meta.HasMore = oas.NewOptBool(true)
		meta.NextCursor = oas.NewOptNilString(strconv.Itoa(offset + limit))
	}
	return items, meta
}

func topologyOf(p topology.Plan) oas.TopologyPreview {
	out := oas.TopologyPreview{Label: p.Label, NodeCount: oas.NewOptInt(p.NodeCount()), Nodes: []oas.TopologyPreviewNodesItem{}, Flows: []oas.TopologyPreviewFlowsItem{}}
	for _, n := range p.Nodes {
		item := oas.TopologyPreviewNodesItem{Role: n.Role, Count: n.Count, Engine: oas.NewOptString(n.Engine)}
		if n.ColocatedWith != "" {
			item.ColocatedWith = oas.NewOptString(n.ColocatedWith)
		}
		out.Nodes = append(out.Nodes, item)
	}
	for _, f := range p.Flows {
		item := oas.TopologyPreviewFlowsItem{From: f.From, To: f.To, Protocol: f.Protocol}
		if f.Port > 0 {
			item.Port = oas.NewOptInt(f.Port)
		}
		out.Flows = append(out.Flows, item)
	}
	return out
}

func requirementsOf(reqs map[string]topology.Requirement) oas.Requirements {
	out := oas.Requirements{}
	for role, r := range reqs {
		out[role] = oas.RoleRequirement{CPU: oas.NewOptInt(r.CPU), MemoryGB: oas.NewOptFloat64(r.MemoryGB), DiskGB: oas.NewOptFloat64(r.DiskGB), Reason: oas.NewOptString(r.Reason)}
	}
	return out
}

func configsOf(configs map[string]map[string]json.RawMessage) map[string]map[string]oas.SchemaValue {
	out := map[string]map[string]oas.SchemaValue{}
	for role, byID := range configs {
		out[role] = map[string]oas.SchemaValue{}
		for id, raw := range byID {
			out[role][id] = schemaValueOf(raw)
		}
	}
	return out
}

func configsFrom(in map[string]map[string]oas.SchemaValue) map[string]map[string]json.RawMessage {
	if in == nil {
		return nil
	}
	out := map[string]map[string]json.RawMessage{}
	for role, byID := range in {
		out[role] = map[string]json.RawMessage{}
		for id, v := range byID {
			out[role][id] = rawOf(v)
		}
	}
	return out
}

func tagsOf(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func tagsFilterOf(s oas.OptString) map[string]string {
	v, ok := s.Get()
	if !ok || v == "" {
		return nil
	}
	out := map[string]string{}
	for _, pair := range splitTags(v) {
		k, val, _ := cutTag(pair)
		out[k] = val
	}
	return out
}

func splitTags(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func cutTag(pair string) (key, value string, ok bool) {
	for i := 0; i < len(pair); i++ {
		if pair[i] == '=' || pair[i] == ':' {
			return pair[:i], pair[i+1:], true
		}
	}
	return pair, "", false
}

func validationErrOf(v any) oas.OptValidationResult {
	if v == nil {
		return oas.OptValidationResult{}
	}
	if err, ok := v.(error); ok {
		var pipelineValidation *spec.ValidationError
		if errors.As(err, &pipelineValidation) {
			return oas.NewOptValidationResult(resourceValidationOf(pipelineValidation.Issues))
		}
		if domain, ok := errs.AsValidation(err); ok {
			if vr, ok := domain.Validation.(*schemapb.ValidationResult); ok {
				return oas.NewOptValidationResult(validationOf(vr))
			}
		}
		return oas.NewOptValidationResult(oas.ValidationResult{Errors: []oas.ValidationError{{Path: "", Code: "INVALID", Message: oas.NewOptString(err.Error())}}})
	}
	return oas.OptValidationResult{}
}

// --- documents --------------------------------------------------------------

func exportDocumentOf(d library.Document) *oas.ExportDocument {
	out := &oas.ExportDocument{APIVersion: d.APIVersion, Kind: oas.ExportDocumentKind(d.Kind), Spec: oas.ExportDocumentSpec{}}
	meta := oas.ExportDocumentMetadata{Name: oas.NewOptString(d.Metadata.Name)}
	if d.Metadata.Description != "" {
		meta.Description = oas.NewOptString(d.Metadata.Description)
	}
	if len(d.Metadata.Tags) > 0 {
		meta.Tags = oas.NewOptExportDocumentMetadataTags(oas.ExportDocumentMetadataTags(d.Metadata.Tags))
	}
	out.Metadata = oas.NewOptExportDocumentMetadata(meta)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(browserSchemaJSON(d.Spec), &m) //nolint:errcheck // built by us
	for k, v := range m {
		out.Spec[k] = jx.Raw(v)
	}
	return out
}

func documentFrom(d *oas.ExportDocument) library.Document {
	out := library.Document{APIVersion: d.APIVersion, Kind: string(d.Kind)}
	if meta, ok := d.Metadata.Get(); ok {
		out.Metadata.Name = meta.Name.Or("")
		out.Metadata.Description = meta.Description.Or("")
		if tags, ok := meta.Tags.Get(); ok {
			out.Metadata.Tags = tags
		}
	}
	out.Spec = rawOf(oas.SchemaValue(d.Spec))
	return out
}

// yamlDocument renders a document as YAML.
func yamlDocument(d library.Document) (io.Reader, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	generic, err := spec.DecodeObject(raw)
	if err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(generic)
	if err != nil {
		return nil, err
	}
	return bytesReader(out), nil
}

// documentFromYAML parses a YAML document.
func documentFromYAML(r io.Reader) (library.Document, error) {
	body, err := io.ReadAll(io.LimitReader(r, 4<<20))
	if err != nil {
		return library.Document{}, err
	}
	var generic map[string]any
	if err := yaml.Unmarshal(body, &generic); err != nil {
		return library.Document{}, errs.Wrap(errs.CodeInvalid, "yaml", err)
	}
	raw, err := json.Marshal(generic)
	if err != nil {
		return library.Document{}, errs.Wrap(errs.CodeInvalid, "yaml", err)
	}
	var d library.Document
	if err := json.Unmarshal(raw, &d); err != nil {
		return library.Document{}, errs.Wrap(errs.CodeInvalid, "document", err)
	}
	return d, nil
}

func diffOf(changes []library.Change) *oas.Diff {
	out := &oas.Diff{Changes: make([]oas.DiffChangesItem, 0, len(changes))}
	for _, c := range changes {
		out.Changes = append(out.Changes, oas.DiffChangesItem{Path: c.Path, Op: oas.DiffChangesItemOp(c.Op), A: jx.Raw(c.A), B: jx.Raw(c.B)})
	}
	return out
}

func usagesOf(us []library.Usage) []oas.Usage {
	out := make([]oas.Usage, 0, len(us))
	for _, u := range us {
		out = append(out, oas.Usage{Kind: oas.UsageKind(u.Kind), ID: u.ID, Name: u.Name})
	}
	return out
}

// documentOf decodes an import body: a JSON document or YAML text.
func documentOf(req any) (library.Document, error) {
	switch r := req.(type) {
	case *oas.ExportDocument:
		return documentFrom(r), nil
	case *oas.ImportDatabaseReqApplicationYaml:
		return documentFromYAML(r.Data)
	case *oas.ImportWorkloadReqApplicationYaml:
		return documentFromYAML(r.Data)
	case *oas.ImportTestReqApplicationYaml:
		return documentFromYAML(r.Data)
	case *oas.ImportSuiteReqApplicationYaml:
		return documentFromYAML(r.Data)
	}
	return library.Document{}, errs.Invalid("unsupported document body")
}

type acceptKey struct{}

// WithAccept stores the request's Accept header for exporters.
func WithAccept(ctx context.Context, accept string) context.Context {
	return context.WithValue(ctx, acceptKey{}, accept)
}

func wantsYAML(ctx context.Context) bool {
	accept, _ := ctx.Value(acceptKey{}).(string) //nolint:errcheck // absent = JSON
	return strings.Contains(accept, "yaml")
}

// AcceptMiddleware passes the Accept header to handlers that pick a
// representation (exports).
func AcceptMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Stroppy-Contract-Version", spec.ContractVersion)
		next.ServeHTTP(w, r.WithContext(WithAccept(r.Context(), r.Header.Get("Accept"))))
	})
}
