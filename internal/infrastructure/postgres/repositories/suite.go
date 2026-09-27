package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/jackc/pgx/v5"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/suite"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// SuiteRepo stores suites, suite runs and the schedule links.
type SuiteRepo struct{ q *db.Queries }

// NewSuiteRepo builds the repository over the ambient-transaction DB.
func NewSuiteRepo(database tx.DB) *SuiteRepo { return &SuiteRepo{q: db.New(database)} }

func (r *SuiteRepo) Insert(ctx context.Context, s suite.Suite) error {
	tests, _ := json.Marshal(s.Tests)   //nolint:errcheck // structs
	axes, _ := json.Marshal(s.Axes)     //nolint:errcheck // structs
	cells, _ := json.Marshal(s.Cells)   //nolint:errcheck // structs
	defs, _ := json.Marshal(s.Defaults) //nolint:errcheck // structs
	err := r.q.InsertSuite(ctx, db.InsertSuiteParams{
		ID: s.ID, TenantID: s.TenantID, Name: s.Name, Description: s.Description, Tags: tagsJSON(s.Tags), AuthorID: s.AuthorID,
		Tests: tests, Axes: axes, Cells: cells, Concurrency: int32(s.Concurrency), Defaults: defs, //nolint:gosec // small
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a suite with this name exists")
		}
		return infraf("suite: insert: %v", err)
	}
	return nil
}

func (r *SuiteRepo) ByID(ctx context.Context, id uuid.UUID) (suite.Suite, error) {
	row, err := r.q.SuiteByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return suite.Suite{}, errs.NotFound("suite")
		}
		return suite.Suite{}, infraf("suite: by id: %v", err)
	}
	return suiteOf(row), nil
}

func (r *SuiteRepo) ByName(ctx context.Context, tenantID uuid.UUID, name string) (uuid.UUID, bool, error) {
	row, err := r.q.SuiteByName(ctx, db.SuiteByNameParams{TenantID: tenantID, Name: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, infraf("suite: by name: %v", err)
	}
	return row.ID, true, nil
}

func (r *SuiteRepo) List(ctx context.Context, tenantID uuid.UUID, q library.ListQuery, favoritesOf string) ([]suite.Suite, error) {
	lim, off := listWindow(q)
	rows, err := r.q.SuitesOfTenant(ctx, db.SuitesOfTenantParams{
		TenantID: tenantID, Search: &q.Search, AuthorID: &q.AuthorID, Tags: tagsFilter(q.Tags), FavoritesOf: &favoritesOf,
		SortKey: &q.Sort, Desc: &q.Desc, Lim: &lim, Off: &off,
	})
	if err != nil {
		return nil, infraf("suite: list: %v", err)
	}
	out := make([]suite.Suite, 0, len(rows))
	for _, row := range rows {
		out = append(out, suiteOf(db.SuiteByIDRow(row)))
	}
	return out, nil
}

func (r *SuiteRepo) Update(ctx context.Context, id uuid.UUID, p suite.Patch) error {
	params := db.UpdateSuiteParams{ID: id, Name: p.Name, Description: p.Description}
	if p.Tags != nil {
		params.Tags = tagsJSON(p.Tags)
	}
	if p.SetTests {
		params.Tests, _ = json.Marshal(p.Tests) //nolint:errcheck // structs
	}
	if p.Axes != nil {
		params.Axes, _ = json.Marshal(p.Axes) //nolint:errcheck // structs
	}
	if p.SetCells {
		params.Cells, _ = json.Marshal(p.Cells) //nolint:errcheck // structs
	}
	if p.Concurrency != nil {
		c := int32(*p.Concurrency) //nolint:gosec // small
		params.Concurrency = &c
	}
	if p.Defaults != nil {
		params.Defaults, _ = json.Marshal(p.Defaults) //nolint:errcheck // structs
	}
	n, err := r.q.UpdateSuite(ctx, params)
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a suite with this name exists")
		}
		return infraf("suite: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("suite")
	}
	return nil
}

func (r *SuiteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteSuite(ctx, id)
	if err != nil {
		return infraf("suite: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("suite")
	}
	return nil
}

func (r *SuiteRepo) UsingTest(ctx context.Context, testID uuid.UUID) ([]library.Usage, error) {
	id := testID.String()
	rows, err := r.q.SuitesUsingTest(ctx, &id)
	if err != nil {
		return nil, infraf("suite: using test: %v", err)
	}
	out := make([]library.Usage, 0, len(rows))
	for _, row := range rows {
		out = append(out, library.Usage{Kind: "suite", ID: row.ID, Name: row.Name})
	}
	return out, nil
}

func suiteOf(row db.SuiteByIDRow) suite.Suite {
	s := suite.Suite{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name, Description: row.Description, AuthorID: row.AuthorID,
		Concurrency: int(row.Concurrency), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Tags: map[string]string{},
	}
	_ = json.Unmarshal(row.Tags, &s.Tags)         //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.Tests, &s.Tests)       //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.Axes, &s.Axes)         //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.Cells, &s.Cells)       //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.Defaults, &s.Defaults) //nolint:errcheck // stored by us
	return s
}

// --- suite runs -------------------------------------------------------------

func (r *SuiteRepo) InsertRun(ctx context.Context, x suite.SuiteRun, idempotencyKey string) error {
	cells, _ := json.Marshal(x.Cells) //nolint:errcheck // structs
	err := r.q.InsertSuiteRun(ctx, db.InsertSuiteRunParams{
		ID: x.ID, TenantID: x.TenantID, SuiteID: x.SuiteID, SuiteName: x.SuiteName, Name: x.Name, Status: string(x.Status), Trigger: string(x.Trigger),
		ScheduleID: x.ScheduleID, RetryOf: x.RetryOf, Concurrency: int32(x.Concurrency), Cells: cells, Labels: tagsJSON(x.Labels), AuthorID: x.AuthorID, //nolint:gosec // small
		GrapheneNamespace: x.GrapheneNamespace, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a suite run with this idempotency key exists")
		}
		return infraf("suite run: insert: %v", err)
	}
	return nil
}

func (r *SuiteRepo) RunByID(ctx context.Context, id uuid.UUID) (suite.SuiteRun, error) {
	row, err := r.q.SuiteRunByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return suite.SuiteRun{}, errs.NotFound("suite run")
		}
		return suite.SuiteRun{}, infraf("suite run: by id: %v", err)
	}
	return suiteRunOf(row), nil
}

func (r *SuiteRepo) RunByIdempotencyKey(ctx context.Context, tenantID uuid.UUID, key string) (uuid.UUID, bool, error) {
	row, err := r.q.SuiteRunByIdempotencyKey(ctx, db.SuiteRunByIdempotencyKeyParams{TenantID: tenantID, Key: key})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, infraf("suite run: idempotency: %v", err)
	}
	return row.ID, true, nil
}

func (r *SuiteRepo) Runs(ctx context.Context, tenantID uuid.UUID, q suite.ListQuery) ([]suite.SuiteRun, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.q.SuiteRunsOfTenant(ctx, db.SuiteRunsOfTenantParams{
		TenantID: tenantID, Statuses: q.Statuses, Triggers: q.Triggers, SuiteID: q.SuiteID, StartedAfter: q.StartedAfter, StartedBefore: q.StartedBefore,
		SortKey: q.Sort, Desc: q.Desc, Lim: int64(limit), Off: int64(q.Offset),
	})
	if err != nil {
		return nil, infraf("suite run: list: %v", err)
	}
	out := make([]suite.SuiteRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, suiteRunOf(db.SuiteRunByIDRow(row)))
	}
	return out, nil
}

func (r *SuiteRepo) RunsOfSuite(ctx context.Context, suiteID uuid.UUID, limit, offset int) ([]suite.SuiteRun, error) {
	rows, err := r.q.SuiteRunsOfSuite(ctx, db.SuiteRunsOfSuiteParams{SuiteID: &suiteID, Lim: ptrInt64(int64(limit)), Off: ptrInt64(int64(offset))})
	if err != nil {
		return nil, infraf("suite run: of suite: %v", err)
	}
	out := make([]suite.SuiteRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, suiteRunOf(db.SuiteRunByIDRow(row)))
	}
	return out, nil
}

func (r *SuiteRepo) RunCountOfSuite(ctx context.Context, suiteID uuid.UUID) (int, error) {
	row, err := r.q.SuiteRunCountOfSuite(ctx, &suiteID)
	if err != nil {
		return 0, infraf("suite run: count: %v", err)
	}
	return int(row.N), nil
}

func (r *SuiteRepo) LiveRuns(ctx context.Context) ([]run.Live, error) {
	rows, err := r.q.LiveSuiteRuns(ctx)
	if err != nil {
		return nil, infraf("suite run: live: %v", err)
	}
	out := make([]run.Live, 0, len(rows))
	for _, row := range rows {
		out = append(out, run.Live{ID: row.ID, TenantID: row.TenantID, Namespace: row.GrapheneNamespace, LastEventID: row.LastEventID, Status: run.Status(row.Status)})
	}
	return out, nil
}

func (r *SuiteRepo) SetRunStatus(ctx context.Context, id uuid.UUID, status run.Status, reason string, startedAt, finishedAt *time.Time) error {
	value := string(status)
	if err := r.q.SetSuiteRunStatus(ctx, db.SetSuiteRunStatusParams{ID: id, Status: &value, StatusReason: &reason, StartedAt: startedAt, FinishedAt: finishedAt}); err != nil {
		return infraf("suite run: set status: %v", err)
	}
	return nil
}

func (r *SuiteRepo) SetRunEvent(ctx context.Context, id uuid.UUID, lastEventID int64) error {
	if err := r.q.SetSuiteRunEvent(ctx, db.SetSuiteRunEventParams{ID: id, LastEventID: &lastEventID}); err != nil {
		return infraf("suite run: set event: %v", err)
	}
	return nil
}

func (r *SuiteRepo) DeleteRun(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteSuiteRun(ctx, id)
	if err != nil {
		return infraf("suite run: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("suite run")
	}
	return nil
}

func (r *SuiteRepo) SchedulesOf(ctx context.Context, kind string, id uuid.UUID) ([]run.Ref, error) {
	rows, err := r.q.SchedulesOfTarget(ctx, db.SchedulesOfTargetParams{TargetKind: kind, TargetID: id})
	if err != nil {
		return nil, infraf("schedules of target: %v", err)
	}
	out := make([]run.Ref, 0, len(rows))
	for _, row := range rows {
		out = append(out, run.Ref{ID: row.ID, Name: row.Name})
	}
	return out, nil
}

func suiteRunOf(row db.SuiteRunByIDRow) suite.SuiteRun {
	x := suite.SuiteRun{
		ID: row.ID, TenantID: row.TenantID, SuiteID: row.SuiteID, SuiteName: row.SuiteName, Name: row.Name, Status: run.Status(row.Status), StatusReason: row.StatusReason,
		Trigger: run.Trigger(row.Trigger), ScheduleID: row.ScheduleID, RetryOf: row.RetryOf, Concurrency: int(row.Concurrency), AuthorID: row.AuthorID,
		GrapheneNamespace: row.GrapheneNamespace, LastEventID: row.LastEventID, CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
		DurationSeconds: row.DurationSeconds, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt, Labels: map[string]string{},
	}
	_ = json.Unmarshal(row.Cells, &x.Cells)   //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.Labels, &x.Labels) //nolint:errcheck // stored by us
	return x
}
