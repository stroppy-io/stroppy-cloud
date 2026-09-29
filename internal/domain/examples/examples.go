// Package examples is the read-only gallery shipped with the server
// (§16.0): documents a tenant clones into its library or quick-runs
// without cloning. Nothing here belongs to a tenant.
package examples

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
)

// Example is one gallery entry.
type Example struct {
	ID          string
	Kind        string // database | workload | test | suite
	Title       string
	Description string
	DBKind      string
	Tags        map[string]string
	Document    library.Document
}

func doc(kind, name, description string, spec any) library.Document {
	raw, _ := json.Marshal(spec) //nolint:errcheck // literals
	return library.Document{APIVersion: library.APIVersion, Kind: kind, Metadata: library.Metadata{Name: name, Description: description}, Spec: raw}
}

// Gallery is the static list.
var Gallery = []Example{
	{
		ID: "pg-single", Kind: "database", Title: "PostgreSQL 17, single node", Description: "One primary, stock configuration — the self-check of a fresh installation.", DBKind: "postgres",
		Tags:     map[string]string{"example": "self-check"},
		Document: doc("Database", "Example: PostgreSQL 17 single", "One primary, stock configuration.", map[string]any{"kind": "postgres", "version": "17", "params": map[string]any{"version": "17"}}),
	},
	{
		ID: "pg-replica-haproxy", Kind: "database", Title: "PostgreSQL 17, primary + replica behind HAProxy", Description: "Streaming replication with a read/write split through HAProxy.", DBKind: "postgres",
		Tags:     map[string]string{"example": "ha"},
		Document: doc("Database", "Example: PostgreSQL 17 primary+replica", "Streaming replica and HAProxy rw/ro listeners.", map[string]any{"kind": "postgres", "version": "17", "params": map[string]any{"version": "17", "replicas": 1, "haproxy": 1}}),
	},
	{
		ID: "cockroach-3", Kind: "database", Title: "CockroachDB, three nodes", Description: "A three-node cluster behind HAProxy.", DBKind: "cockroach",
		Tags:     map[string]string{"example": "distributed"},
		Document: doc("Database", "Example: CockroachDB ×3", "Three nodes, HAProxy in front.", map[string]any{"kind": "cockroach", "version": "25.4", "params": map[string]any{"version": "25.4", "nodes": 3, "haproxy": 1}}),
	},
	{
		ID: "tpcc-smoke", Kind: "workload", Title: "TPC-C smoke, 5 minutes", Description: "Scale factor 1, 8 virtual users, five minutes — a quick functional pass.",
		Tags: map[string]string{"example": "smoke", "script": "tpcc/tx"},
		Document: doc("Workload", "Example: TPC-C smoke", "Scale factor 1, 8 VUs, 5 minutes.", map[string]any{
			"stroppy_version": "6.1.0", "protocol": "pg",
			"segments": []any{map[string]any{"name": "main", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 1}, "run": map[string]any{"vus": 8, "duration": "5m"}}},
		}),
	},
	{
		ID: "tpcc-standard", Kind: "workload", Title: "TPC-C, 50 warehouses, 30 minutes", Description: "The reference TPC-C pass: scale factor 50, 64 virtual users.",
		Tags: map[string]string{"example": "benchmark", "script": "tpcc/tx"},
		Document: doc("Workload", "Example: TPC-C 50 warehouses", "Scale factor 50, 64 VUs, 30 minutes.", map[string]any{
			"stroppy_version": "6.1.0", "protocol": "pg",
			"segments": []any{map[string]any{"name": "main", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 50}, "run": map[string]any{"vus": 64, "duration": "30m"}}},
		}),
	},
	{
		ID: "tpch-split", Kind: "workload", Title: "TPC-H, load then queries", Description: "The eight-table load as one segment and the 22-query suite as another.",
		Tags: map[string]string{"example": "analytics", "script": "tpch/tx"},
		Document: doc("Workload", "Example: TPC-H split", "Load segment, then the query suite.", map[string]any{
			"stroppy_version": "6.1.0", "protocol": "pg",
			"segments": []any{
				map[string]any{"name": "load", "workload": map[string]any{"script": "tpch/tx", "scale_factor": 1}, "run": map[string]any{"executor": "shared-iterations", "vus": 4, "iterations": 1}},
				map[string]any{"name": "queries", "workload": map[string]any{"script": "tpch/tx", "scale_factor": 1}, "run": map[string]any{"vus": 4, "duration": "10m"}},
			},
		}),
	},
	{
		ID: "pg-self-check", Kind: "test", Title: "PostgreSQL self-check", Description: "Single-node PostgreSQL with the TPC-C smoke: pick a provider profile and go.", DBKind: "postgres",
		Tags: map[string]string{"example": "self-check"},
		Document: doc("Test", "Example: PostgreSQL self-check", "Single node + TPC-C smoke.", map[string]any{
			"database": map[string]any{"inline": map[string]any{"kind": "postgres", "version": "17", "params": map[string]any{"version": "17"}}},
			"workload": map[string]any{"inline": map[string]any{"stroppy_version": "6.1.0", "protocol": "pg", "segments": []any{map[string]any{"name": "main", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 1}, "run": map[string]any{"vus": 2, "duration": "5m"}}}}},
			"sizes":    map[string]any{"db": map[string]any{"size": "S"}, "runner": map[string]any{"size": "S"}},
		}),
	},
	{
		ID: "pg-vs-cockroach", Kind: "suite", Title: "PostgreSQL vs CockroachDB", Description: "The same TPC-C smoke on a PostgreSQL single node and a three-node CockroachDB cluster, two sizes each.",
		Tags: map[string]string{"example": "comparison"},
		Document: doc("Suite", "Example: PostgreSQL vs CockroachDB", "TPC-C smoke on both engines, S and M.", map[string]any{
			"tests": []any{
				map[string]any{"inline": map[string]any{
					"database_inline": map[string]any{"kind": "postgres", "version": "17", "params": map[string]any{"version": "17"}},
					"workload_inline": map[string]any{"stroppy_version": "6.1.0", "protocol": "pg", "segments": []any{map[string]any{"name": "main", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 1}, "run": map[string]any{"vus": 8, "duration": "5m"}}}},
					"rating_tenant":   true,
				}, "inline_name": "postgres"},
				map[string]any{"inline": map[string]any{
					"database_inline": map[string]any{"kind": "cockroach", "version": "25.4", "params": map[string]any{"version": "25.4", "nodes": 3}},
					"workload_inline": map[string]any{"stroppy_version": "6.1.0", "protocol": "cockroach", "segments": []any{map[string]any{"name": "main", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 1}, "run": map[string]any{"vus": 8, "duration": "5m"}}}},
					"rating_tenant":   true,
				}, "inline_name": "cockroach"},
			},
			"axes":        map[string]any{"sizes": []any{map[string]any{"db": map[string]any{"size": "S"}, "runner": map[string]any{"size": "S"}}, map[string]any{"db": map[string]any{"size": "M"}, "runner": map[string]any{"size": "S"}}}},
			"concurrency": 1,
		}),
	},
}

// Suites imports suite documents (the suite domain).
type Suites interface {
	ImportDocument(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, doc library.Document) ([]library.Usage, error)
}

// Service is the gallery use cases.
type Service struct {
	library *library.Service
	suites  Suites
}

// NewService wires the use cases.
func NewService(lib *library.Service, suites Suites) *Service {
	return &Service{library: lib, suites: suites}
}

// List filters the gallery.
func (s *Service) List(kind, dbKind string) []Example {
	out := make([]Example, 0, len(Gallery))
	for _, e := range Gallery {
		if (kind == "" || e.Kind == kind) && (dbKind == "" || e.DBKind == dbKind) {
			out = append(out, e)
		}
	}
	return out
}

// Get finds an example.
func (s *Service) Get(id string) (Example, bool) {
	for _, e := range Gallery {
		if e.ID == id {
			return e, true
		}
	}
	return Example{}, false
}

// Clone imports the example's document into the tenant's library under
// the given name (default: the document's).
func (s *Service) Clone(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, id, name string) ([]library.Usage, error) {
	e, ok := s.Get(id)
	if !ok {
		return nil, errs.NotFound("example")
	}
	d := e.Document
	if name != "" {
		d.Metadata.Name = name
	}
	switch e.Kind {
	case "database":
		x, _, _, err := s.library.ImportDatabase(ctx, actor, tenantID, d)
		if err != nil {
			return nil, err
		}
		return []library.Usage{{Kind: "database", ID: x.ID, Name: x.Name}}, nil
	case "workload":
		x, _, _, err := s.library.ImportWorkload(ctx, actor, tenantID, d)
		if err != nil {
			return nil, err
		}
		return []library.Usage{{Kind: "workload", ID: x.ID, Name: x.Name}}, nil
	case "test":
		x, _, _, _, err := s.library.ImportTest(ctx, actor, tenantID, d)
		if err != nil {
			return nil, err
		}
		return []library.Usage{{Kind: "test", ID: x.ID, Name: x.Name}}, nil
	case "suite":
		if s.suites == nil {
			return nil, errs.New(errs.CodeUnavailable, "suites are not available")
		}
		return s.suites.ImportDocument(ctx, actor, tenantID, d)
	default:
		return nil, errs.Invalid("example kind " + e.Kind)
	}
}

// TestSpec resolves an example test's spec for a quick run (no clone).
func (s *Service) TestSpec(ctx context.Context, tenantID uuid.UUID, id string) (library.TestSpec, string, error) {
	e, ok := s.Get(id)
	if !ok {
		return library.TestSpec{}, "", errs.NotFound("example")
	}
	if e.Kind != "test" {
		return library.TestSpec{}, "", errs.Invalid("only test examples can be quick-run")
	}
	spec, err := s.library.TestSpecFromDocument(ctx, tenantID, e.Document)
	if err != nil {
		return library.TestSpec{}, "", err
	}
	return spec, e.Document.Metadata.Name, nil
}
