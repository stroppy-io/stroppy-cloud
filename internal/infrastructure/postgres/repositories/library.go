package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// LibraryRepo stores databases, workloads and tests.
type LibraryRepo struct {
	q *db.Queries
}

var _ library.Repository = (*LibraryRepo)(nil)

// NewLibraryRepo builds the repo.
func NewLibraryRepo(database tx.DB) *LibraryRepo { return &LibraryRepo{q: db.New(database)} }

func tagsJSON(tags map[string]string) json.RawMessage {
	if tags == nil {
		tags = map[string]string{}
	}
	raw, _ := json.Marshal(tags) //nolint:errcheck // map[string]string always marshals
	return raw
}

func tagsFilter(tags map[string]string) json.RawMessage {
	if len(tags) == 0 {
		return nil
	}
	return tagsJSON(tags)
}

func orEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

func entityOf(id, tenantID uuid.UUID, name, description string, tags json.RawMessage, author *uuid.UUID, created, updated time.Time) library.Entity {
	e := library.Entity{ID: id, TenantID: tenantID, Name: name, Description: description, AuthorID: author, CreatedAt: created, UpdatedAt: updated, Tags: map[string]string{}}
	_ = json.Unmarshal(tags, &e.Tags) //nolint:errcheck // stored by us
	return e
}

func listWindow(q library.ListQuery) (limit, offset int64) {
	l := q.Limit
	if l <= 0 || l > 200 {
		l = 50
	}
	return int64(l), int64(q.Offset)
}

// --- databases --------------------------------------------------------------

func (r *LibraryRepo) InsertDatabase(ctx context.Context, d library.Database) error {
	configs, _ := json.Marshal(d.Spec.Configs)                                //nolint:errcheck // map always marshals
	external, _ := json.Marshal(map[string]string{"dsn": d.Spec.ExternalDSN}) //nolint:errcheck // map always marshals
	err := r.q.InsertDatabase(ctx, db.InsertDatabaseParams{
		ID: d.ID, TenantID: d.TenantID, Name: d.Name, Description: d.Description, Tags: tagsJSON(d.Tags), AuthorID: d.AuthorID,
		Kind: string(d.Spec.Kind), Version: d.Spec.Version, Image: d.Spec.Image, Params: orEmpty(d.Spec.Params), Configs: orEmpty(configs), External: external, Runtime: orEmpty(d.Spec.Runtime),
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a database with this name exists")
		}
		return infraf("database: insert: %v", err)
	}
	return nil
}

func (r *LibraryRepo) DatabaseByID(ctx context.Context, id uuid.UUID) (library.Database, error) {
	row, err := r.q.DatabaseByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return library.Database{}, errs.NotFound("database")
		}
		return library.Database{}, infraf("database: by id: %v", err)
	}
	return databaseOf(row), nil
}

func (r *LibraryRepo) Databases(ctx context.Context, tenantID uuid.UUID, q library.ListQuery) ([]library.Database, error) {
	lim, off := listWindow(q)
	rows, err := r.q.DatabasesOfTenant(ctx, db.DatabasesOfTenantParams{
		TenantID: tenantID, Search: q.Search, AuthorID: q.AuthorID, Kinds: q.Kinds, Tags: tagsFilter(q.Tags),
		SortKey: q.Sort, Desc: q.Desc, Lim: lim, Off: off,
	})
	if err != nil {
		return nil, infraf("database: list: %v", err)
	}
	out := make([]library.Database, 0, len(rows))
	for _, row := range rows {
		out = append(out, databaseOf(db.DatabaseByIDRow(row)))
	}
	return out, nil
}

func (r *LibraryRepo) UpdateDatabase(ctx context.Context, id uuid.UUID, p library.EntityPatch, spec *library.DatabaseSpec) error {
	params := db.UpdateDatabaseParams{ID: id, Name: p.Name, Description: p.Description, Tags: tagsFilter(p.Tags)}
	if p.Tags != nil {
		params.Tags = tagsJSON(p.Tags)
	}
	if spec != nil {
		params.Version, params.Image = &spec.Version, &spec.Image
		params.Params = orEmpty(spec.Params)
		params.Runtime = orEmpty(spec.Runtime)
		configs, _ := json.Marshal(spec.Configs) //nolint:errcheck // map always marshals
		params.Configs = orEmpty(configs)
	}
	n, err := r.q.UpdateDatabase(ctx, params)
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a database with this name exists")
		}
		return infraf("database: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("database")
	}
	return nil
}

func (r *LibraryRepo) DeleteDatabase(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteDatabase(ctx, id)
	if err != nil {
		return infraf("database: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("database")
	}
	return nil
}

func (r *LibraryRepo) TestsUsingDatabase(ctx context.Context, id uuid.UUID) ([]library.Usage, error) {
	rows, err := r.q.TestsUsingDatabase(ctx, &id)
	if err != nil {
		return nil, infraf("database: usages: %v", err)
	}
	out := make([]library.Usage, 0, len(rows))
	for _, row := range rows {
		out = append(out, library.Usage{Kind: "test", ID: row.ID, Name: row.Name})
	}
	return out, nil
}

func (r *LibraryRepo) InlineDatabase(ctx context.Context, id uuid.UUID, spec library.DatabaseSpec) error {
	raw, _ := json.Marshal(spec) //nolint:errcheck // struct
	if _, err := r.q.InlineDatabaseIntoTests(ctx, db.InlineDatabaseIntoTestsParams{DatabaseID: &id, Spec: raw}); err != nil {
		return infraf("database: inline: %v", err)
	}
	return nil
}

func databaseOf(row db.DatabaseByIDRow) library.Database {
	d := library.Database{
		Entity: entityOf(row.ID, row.TenantID, row.Name, row.Description, row.Tags, row.AuthorID, row.CreatedAt, row.UpdatedAt),
		Spec:   library.DatabaseSpec{Runtime: row.Runtime, Kind: libraryKind(row.Kind), Version: row.Version, Image: row.Image, Params: row.Params},
	}
	_ = json.Unmarshal(row.Configs, &d.Spec.Configs) //nolint:errcheck // stored by us
	var ext struct {
		DSN string `json:"dsn"`
	}
	_ = json.Unmarshal(row.External, &ext) //nolint:errcheck // stored by us
	d.Spec.ExternalDSN = ext.DSN
	return d
}

// --- workloads --------------------------------------------------------------

func (r *LibraryRepo) InsertWorkload(ctx context.Context, w library.Workload) error {
	err := r.q.InsertWorkload(ctx, db.InsertWorkloadParams{
		ID: w.ID, TenantID: w.TenantID, Name: w.Name, Description: w.Description, Tags: tagsJSON(w.Tags), AuthorID: w.AuthorID,
		StroppyVersion: w.Spec.StroppyVersion, Protocol: string(w.Spec.Protocol), Spec: orEmpty(w.Baked),
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a workload with this name exists")
		}
		return infraf("workload: insert: %v", err)
	}
	return nil
}

func (r *LibraryRepo) WorkloadByID(ctx context.Context, id uuid.UUID) (library.Workload, error) {
	row, err := r.q.WorkloadByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return library.Workload{}, errs.NotFound("workload")
		}
		return library.Workload{}, infraf("workload: by id: %v", err)
	}
	return workloadOf(row), nil
}

func (r *LibraryRepo) Workloads(ctx context.Context, tenantID uuid.UUID, q library.ListQuery) ([]library.Workload, error) {
	lim, off := listWindow(q)
	rows, err := r.q.WorkloadsOfTenant(ctx, db.WorkloadsOfTenantParams{
		TenantID: tenantID, Search: q.Search, AuthorID: q.AuthorID, Protocols: q.Protocols, Versions: q.Versions, Script: q.Script,
		Tags: tagsFilter(q.Tags), SortKey: q.Sort, Desc: q.Desc, Lim: lim, Off: off,
	})
	if err != nil {
		return nil, infraf("workload: list: %v", err)
	}
	out := make([]library.Workload, 0, len(rows))
	for _, row := range rows {
		out = append(out, workloadOf(db.WorkloadByIDRow(row)))
	}
	return out, nil
}

func (r *LibraryRepo) UpdateWorkload(ctx context.Context, id uuid.UUID, p library.EntityPatch, spec *library.WorkloadSpec, baked json.RawMessage) error {
	params := db.UpdateWorkloadParams{ID: id, Name: p.Name, Description: p.Description}
	if p.Tags != nil {
		params.Tags = tagsJSON(p.Tags)
	}
	if spec != nil {
		proto := string(spec.Protocol)
		params.StroppyVersion, params.Protocol, params.Spec = &spec.StroppyVersion, &proto, orEmpty(baked)
	}
	n, err := r.q.UpdateWorkload(ctx, params)
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a workload with this name exists")
		}
		return infraf("workload: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("workload")
	}
	return nil
}

func (r *LibraryRepo) DeleteWorkload(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteWorkload(ctx, id)
	if err != nil {
		return infraf("workload: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("workload")
	}
	return nil
}

func (r *LibraryRepo) TestsUsingWorkload(ctx context.Context, id uuid.UUID) ([]library.Usage, error) {
	rows, err := r.q.TestsUsingWorkload(ctx, &id)
	if err != nil {
		return nil, infraf("workload: usages: %v", err)
	}
	out := make([]library.Usage, 0, len(rows))
	for _, row := range rows {
		out = append(out, library.Usage{Kind: "test", ID: row.ID, Name: row.Name})
	}
	return out, nil
}

func (r *LibraryRepo) InlineWorkload(ctx context.Context, id uuid.UUID, spec library.WorkloadSpec) error {
	raw, _ := json.Marshal(spec) //nolint:errcheck // struct
	if _, err := r.q.InlineWorkloadIntoTests(ctx, db.InlineWorkloadIntoTestsParams{WorkloadID: &id, Spec: raw}); err != nil {
		return infraf("workload: inline: %v", err)
	}
	return nil
}

// workloadOf rebuilds the split spec from the stored baked value.
func workloadOf(row db.WorkloadByIDRow) library.Workload {
	w := library.Workload{
		Entity: entityOf(row.ID, row.TenantID, row.Name, row.Description, row.Tags, row.AuthorID, row.CreatedAt, row.UpdatedAt),
		Spec:   library.WorkloadSpec{StroppyVersion: row.StroppyVersion, Protocol: libraryProtocol(row.Protocol)},
		Baked:  row.Spec,
	}
	var full map[string]json.RawMessage
	_ = json.Unmarshal(row.Spec, &full)                    //nolint:errcheck // stored by us
	_ = json.Unmarshal(full["segments"], &w.Spec.Segments) //nolint:errcheck // stored by us
	delete(full, "segments")
	delete(full, "stroppy_version")
	delete(full, "protocol")
	if len(full) > 0 {
		w.Spec.Options, _ = json.Marshal(full) //nolint:errcheck // map of raw
	}
	return w
}

// --- tests ------------------------------------------------------------------

func (r *LibraryRepo) InsertTest(ctx context.Context, t library.Test) error {
	sizes, _ := json.Marshal(t.Spec.Sizes) //nolint:errcheck // map
	err := r.q.InsertTest(ctx, db.InsertTestParams{
		ID: t.ID, TenantID: t.TenantID, Name: t.Name, Description: t.Description, Tags: tagsJSON(t.Tags), AuthorID: t.AuthorID,
		DatabaseID: t.Spec.DatabaseRef, DatabaseInline: specJSON(t.Spec.DatabaseInline),
		WorkloadID: t.Spec.WorkloadRef, WorkloadInline: specJSON(t.Spec.WorkloadInline),
		Sizes: orEmpty(sizes), Execution: orEmpty(t.Spec.Execution), ProviderProfileID: t.Spec.ProviderProfileID, Keep: keepText(t.Spec.Keep),
		RatingTenant: t.Spec.RatingTenant, RatingGlobal: t.Spec.RatingGlobal, Status: string(t.Status), ValidatedAt: t.ValidatedAt,
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a test with this name exists")
		}
		return infraf("test: insert: %v", err)
	}
	return nil
}

func specJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case *library.DatabaseSpec:
		if s == nil {
			return nil
		}
	case *library.WorkloadSpec:
		if s == nil {
			return nil
		}
	}
	raw, _ := json.Marshal(v) //nolint:errcheck // struct
	return raw
}

func keepText(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.String()
}

func (r *LibraryRepo) TestByID(ctx context.Context, id uuid.UUID) (library.Test, error) {
	row, err := r.q.TestByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return library.Test{}, errs.NotFound("test")
		}
		return library.Test{}, infraf("test: by id: %v", err)
	}
	return testOf(row), nil
}

func (r *LibraryRepo) Tests(ctx context.Context, tenantID uuid.UUID, q library.ListQuery) ([]library.Test, error) {
	lim, off := listWindow(q)
	rows, err := r.q.TestsOfTenant(ctx, db.TestsOfTenantParams{
		TenantID: tenantID, Search: q.Search, AuthorID: q.AuthorID, Statuses: q.Statuses, Tags: tagsFilter(q.Tags),
		SortKey: q.Sort, Desc: q.Desc, Lim: lim, Off: off,
	})
	if err != nil {
		return nil, infraf("test: list: %v", err)
	}
	out := make([]library.Test, 0, len(rows))
	for _, row := range rows {
		out = append(out, testOf(db.TestByIDRow(row)))
	}
	return out, nil
}

func (r *LibraryRepo) UpdateTest(ctx context.Context, t library.Test, p library.TestPatch) error {
	params := db.UpdateTestParams{
		ID: t.ID, Name: p.Name, Description: p.Description, Status: string(t.Status), ValidatedAt: t.ValidatedAt,
		SetDatabase: &p.SetDatabase, DatabaseID: p.DatabaseRef, DatabaseInline: specJSON(p.DatabaseInline),
		SetWorkload: &p.SetWorkload, WorkloadID: p.WorkloadRef, WorkloadInline: specJSON(p.WorkloadInline),
		SetProvider: &p.SetProvider, ProviderProfileID: p.ProviderProfileID,
		RatingTenant: p.RatingTenant, RatingGlobal: p.RatingGlobal, Execution: p.Execution,
	}
	if p.Tags != nil {
		params.Tags = tagsJSON(p.Tags)
	}
	if p.Sizes != nil {
		params.Sizes, _ = json.Marshal(p.Sizes) //nolint:errcheck // map
	}
	if p.Keep != nil {
		k := keepText(*p.Keep)
		params.Keep = &k
	}
	n, err := r.q.UpdateTest(ctx, params)
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a test with this name exists")
		}
		return infraf("test: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("test")
	}
	return nil
}

func (r *LibraryRepo) SetTestStatus(ctx context.Context, id uuid.UUID, status library.TestStatus, validatedAt *time.Time) error {
	if err := r.q.SetTestStatus(ctx, db.SetTestStatusParams{ID: id, Status: string(status), ValidatedAt: validatedAt}); err != nil {
		return infraf("test: set status: %v", err)
	}
	return nil
}

func (r *LibraryRepo) DeleteTest(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteTest(ctx, id)
	if err != nil {
		return infraf("test: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("test")
	}
	return nil
}

func testOf(row db.TestByIDRow) library.Test {
	t := library.Test{
		Entity: entityOf(row.ID, row.TenantID, row.Name, row.Description, row.Tags, row.AuthorID, row.CreatedAt, row.UpdatedAt),
		Spec: library.TestSpec{
			Execution:   row.Execution,
			DatabaseRef: row.DatabaseID, WorkloadRef: row.WorkloadID, ProviderProfileID: row.ProviderProfileID,
			RatingTenant: row.RatingTenant, RatingGlobal: row.RatingGlobal, Sizes: map[string]library.RoleSize{},
		},
		Status: library.TestStatus(row.Status), ValidatedAt: row.ValidatedAt,
	}
	if len(row.DatabaseInline) > 0 && string(row.DatabaseInline) != "null" {
		var s library.DatabaseSpec
		if json.Unmarshal(row.DatabaseInline, &s) == nil {
			t.Spec.DatabaseInline = &s
		}
	}
	if len(row.WorkloadInline) > 0 && string(row.WorkloadInline) != "null" {
		var s library.WorkloadSpec
		if json.Unmarshal(row.WorkloadInline, &s) == nil {
			t.Spec.WorkloadInline = &s
		}
	}
	_ = json.Unmarshal(row.Sizes, &t.Spec.Sizes) //nolint:errcheck // stored by us
	if row.Keep != "" {
		t.Spec.Keep, _ = time.ParseDuration(row.Keep) //nolint:errcheck // stored by us
	}
	return t
}
