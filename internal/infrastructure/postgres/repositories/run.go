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
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// RunRepo stores runs, their timeline and favorites.
type RunRepo struct {
	q  *db.Queries
	tx lifecycleTransactor
}

var _ run.Repository = (*RunRepo)(nil)

// NewRunRepo builds the repo.
func NewRunRepo(database tx.DB, tr lifecycleTransactor) *RunRepo {
	return &RunRepo{q: db.New(database), tx: tr}
}

func (r *RunRepo) Insert(ctx context.Context, x run.Run, idempotencyKey string) error {
	return r.tx.Do(ctx, func(ctx context.Context) error {
		tenant, err := r.q.LockTenantLifecycle(ctx, x.TenantID)
		if err != nil {
			return err
		}
		if tenant.Retiring {
			return errs.Conflict("tenant is being deleted")
		}
		profile, err := r.q.LockProviderLifecycle(ctx, x.Snapshot.ProviderProfile.ID)
		if err != nil {
			return errs.Conflict("provider is unavailable")
		}
		if profile.Status != "ready" {
			return errs.Conflict("provider is not ready")
		}
		return r.insert(ctx, x, idempotencyKey)
	})
}

func (r *RunRepo) insert(ctx context.Context, x run.Run, idempotencyKey string) error {
	snapshot, _ := json.Marshal(x.Snapshot) //nolint:errcheck // struct
	summary, _ := json.Marshal(x.Summary)   //nolint:errcheck // struct
	labels := tagsJSON(x.Labels)
	err := r.q.InsertRun(ctx, db.InsertRunParams{
		ID: x.ID, TenantID: x.TenantID, Name: x.Name, Status: string(x.Status), Phase: string(x.Phase), Trigger: string(x.Trigger),
		SuiteRunID: x.SuiteRunID, CellID: x.CellID, ScheduleID: x.ScheduleID, ParentRunID: x.ParentRunID, TestID: x.TestID, TestName: x.TestName,
		AuthorID: x.AuthorID, Snapshot: snapshot, RunSpec: orEmpty(x.RunSpec), Summary: summary, RatingTenant: x.RatingTenant, RatingGlobal: x.RatingGlobal,
		Keep: keepText(x.Keep), Notes: x.Notes, Labels: labels, GrapheneNamespace: x.GrapheneNamespace, GrapheneRunID: x.GrapheneRunID, PipelineRevision: x.PipelineRevision, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a run with this idempotency key exists")
		}
		return infraf("run: insert: %v", err)
	}
	return nil
}

func (r *RunRepo) ByID(ctx context.Context, id uuid.UUID) (run.Run, error) {
	row, err := r.q.RunByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return run.Run{}, errs.NotFound("run")
		}
		return run.Run{}, infraf("run: by id: %v", err)
	}
	return runOf(row), nil
}

func (r *RunRepo) ByIdempotencyKey(ctx context.Context, tenantID uuid.UUID, key string) (uuid.UUID, bool, error) {
	row, err := r.q.RunByIdempotencyKey(ctx, db.RunByIdempotencyKeyParams{TenantID: tenantID, Key: key})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, infraf("run: by key: %v", err)
	}
	return row.ID, true, nil
}

func (r *RunRepo) List(ctx context.Context, tenantID uuid.UUID, q run.ListQuery) ([]run.Run, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.q.RunsOfTenant(ctx, db.RunsOfTenantParams{
		TenantID: tenantID, Search: q.Search, AuthorID: q.AuthorID, Statuses: q.Statuses, Kinds: q.Kinds, Profiles: q.Profiles, Triggers: q.Triggers,
		TestID: q.TestID, SuiteRunID: q.SuiteRunID, Standalone: q.Standalone, Labels: tagsFilter(q.Labels),
		StartedAfter: q.StartedAfter, StartedBefore: q.StartedBefore, FinishedAfter: q.FinishedAfter, FinishedBefore: q.FinishedBefore,
		DurationMin: q.DurationMin.Seconds(), DurationMax: q.DurationMax.Seconds(), StandKeptOnly: q.StandKeptOnly, FavoritesOf: q.FavoritesOf,
		SortKey: q.Sort, Desc: q.Desc, Lim: int64(limit), Off: int64(q.Offset),
	})
	if err != nil {
		return nil, infraf("run: list: %v", err)
	}
	out := make([]run.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runOf(db.RunByIDRow(row)))
	}
	return out, nil
}

func (r *RunRepo) Facets(ctx context.Context, tenantID uuid.UUID) ([]run.Facet, error) {
	rows, err := r.q.RunFacetValues(ctx, tenantID)
	if err != nil {
		return nil, infraf("run: facets: %v", err)
	}
	var out []run.Facet
	for _, row := range rows {
		if len(out) == 0 || out[len(out)-1].Field != row.Field {
			out = append(out, run.Facet{Field: row.Field, Values: []run.FacetValue{}})
		}
		out[len(out)-1].Values = append(out[len(out)-1].Values, run.FacetValue{Value: row.Value, Count: int(row.Count)})
	}
	return out, nil
}

func (r *RunRepo) OfTest(ctx context.Context, testID uuid.UUID, limit, offset int) ([]run.Run, error) {
	rows, err := r.q.RunsOfTest(ctx, db.RunsOfTestParams{TestID: &testID, Lim: ptrInt64(int64(limit)), Off: ptrInt64(int64(offset))})
	if err != nil {
		return nil, infraf("run: of test: %v", err)
	}
	out := make([]run.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runOf(db.RunByIDRow(row)))
	}
	return out, nil
}

func (r *RunRepo) OfSuiteRun(ctx context.Context, suiteRunID uuid.UUID) ([]run.Run, error) {
	rows, err := r.q.RunsOfSuiteRun(ctx, &suiteRunID)
	if err != nil {
		return nil, infraf("run: of suite run: %v", err)
	}
	out := make([]run.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runOf(db.RunByIDRow(row)))
	}
	return out, nil
}

func (r *RunRepo) ByIDs(ctx context.Context, ids []uuid.UUID) ([]run.Run, error) {
	rows, err := r.q.RunsByIds(ctx, db.RunsByIdsParams{Ids: ids})
	if err != nil {
		return nil, infraf("run: by ids: %v", err)
	}
	out := make([]run.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runOf(db.RunByIDRow(row)))
	}
	return out, nil
}

func (r *RunRepo) Counts(ctx context.Context, tenantID uuid.UUID) (run.Counts, error) {
	row, err := r.q.RunCountsOfTenant(ctx, tenantID)
	if err != nil {
		return run.Counts{}, infraf("run: counts: %v", err)
	}
	return run.Counts{Total: int(row.Total), Pending: int(row.Pending), Running: int(row.Running), Completed: int(row.Completed), Failed: int(row.Failed), Cancelled: int(row.Cancelled), KeptStands: int(row.KeptStands)}, nil
}

func (r *RunRepo) Rating(ctx context.Context, q run.RatingQuery) ([]run.RatingRow, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.q.RatingRuns(ctx, db.RatingRunsParams{
		Metric: q.Metric, TenantID: q.TenantID, Kinds: q.Kinds, Versions: q.Versions, Providers: q.Providers, StroppyVersions: q.StroppyVersions,
		League: q.League, Since: q.Since, HigherIsBetter: q.HigherIsBetter, Lim: int64(limit), Off: int64(q.Offset),
	})
	if err != nil {
		return nil, infraf("run: rating: %v", err)
	}
	out := make([]run.RatingRow, 0, len(rows))
	for _, row := range rows {
		x := run.Run{ID: row.ID, TenantID: row.TenantID, Name: row.Name, AuthorID: row.AuthorID, FinishedAt: row.FinishedAt, TPS: row.Tps}
		_ = json.Unmarshal(row.Summary, &x.Summary) //nolint:errcheck // stored by us
		item := run.RatingRow{Run: x, TenantName: row.TenantName}
		if row.TenantPublicName != nil {
			item.TenantPublicName = *row.TenantPublicName
		}
		if row.League != nil {
			item.League = *row.League
		}
		if row.Value != nil {
			item.Value = *row.Value
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *RunRepo) Leagues(ctx context.Context, tenantID string) ([]string, error) {
	rows, err := r.q.RatingLeagues(ctx, &tenantID)
	if err != nil {
		return nil, infraf("run: leagues: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.League != nil && *row.League != "" {
			out = append(out, *row.League)
		}
	}
	return out, nil
}

func (r *RunRepo) LiveRuns(ctx context.Context) ([]run.Live, error) {
	rows, err := r.q.LiveRuns(ctx)
	if err != nil {
		return nil, infraf("run: live: %v", err)
	}
	out := make([]run.Live, 0, len(rows))
	for _, row := range rows {
		out = append(out, run.Live{ID: row.ID, TenantID: row.TenantID, Namespace: row.GrapheneNamespace, LastEventID: row.LastEventID, Status: run.Status(row.Status)})
	}
	return out, nil
}

func (r *RunRepo) LiveCount(ctx context.Context, tenantID uuid.UUID) (int, error) {
	row, err := r.q.TenantLiveRunCount(ctx, tenantID)
	if err != nil {
		return 0, infraf("run: live count: %v", err)
	}
	return int(row.N), nil
}

func (r *RunRepo) HasLive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	row, err := r.q.TenantHasLiveRuns(ctx, tenantID)
	if err != nil {
		return false, infraf("run: has live: %v", err)
	}
	return row.Live != nil && *row.Live, nil
}

func (r *RunRepo) UpdateMeta(ctx context.Context, id uuid.UUID, p run.MetaPatch) error {
	params := db.UpdateRunMetaParams{ID: id, Name: p.Name, Notes: p.Notes, RatingTenant: p.RatingTenant, RatingGlobal: p.RatingGlobal}
	if p.Labels != nil {
		params.Labels = tagsJSON(p.Labels)
	}
	n, err := r.q.UpdateRunMeta(ctx, params)
	if err != nil {
		return infraf("run: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("run")
	}
	return nil
}

func (r *RunRepo) SetStatus(ctx context.Context, id uuid.UUID, status run.Status, phase run.Phase, reason string, startedAt, finishedAt *time.Time) error {
	if err := r.q.SetRunStatus(ctx, db.SetRunStatusParams{ID: id, Status: string(status), Phase: string(phase), StatusReason: reason, StartedAt: startedAt, FinishedAt: finishedAt}); err != nil {
		return infraf("run: set status: %v", err)
	}
	return nil
}

func (r *RunRepo) SetProjection(ctx context.Context, id uuid.UUID, state run.State, lastEventID int64) error {
	if err := r.q.SetRunProjection(ctx, db.SetRunProjectionParams{ID: id, RuntimeState: state.JSON(), LastEventID: &lastEventID}); err != nil {
		return infraf("run: set projection: %v", err)
	}
	return nil
}

func (r *RunRepo) SetResult(ctx context.Context, id uuid.UUID, result json.RawMessage, summary run.Summary, tps *float64) error {
	raw, _ := json.Marshal(summary) //nolint:errcheck // struct
	if err := r.q.SetRunResult(ctx, db.SetRunResultParams{ID: id, Result: orEmpty(result), Summary: raw, Tps: tps}); err != nil {
		return infraf("run: set result: %v", err)
	}
	return nil
}

func (r *RunRepo) SetKeep(ctx context.Context, id uuid.UUID, kept bool, until *time.Time) error {
	if err := r.q.SetRunKeep(ctx, db.SetRunKeepParams{ID: id, StandKept: kept, KeepUntil: until}); err != nil {
		return infraf("run: set keep: %v", err)
	}
	return nil
}

func (r *RunRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteRun(ctx, id)
	if err != nil {
		return infraf("run: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("run")
	}
	return nil
}

func (r *RunRepo) InsertEvent(ctx context.Context, e run.Event) (bool, error) {
	payload, _ := json.Marshal(e.Payload) //nolint:errcheck // map
	n, err := r.q.InsertRunEvent(ctx, db.InsertRunEventParams{
		RunID: e.RunID, GrapheneID: e.GrapheneID, At: e.At, Kind: e.Kind, Title: e.Title, Subject: e.Subject, Status: e.Status,
		Error: e.Error, Attempt: int32(e.Attempt), Payload: orEmpty(payload), //nolint:gosec // small
	})
	if err != nil {
		return false, infraf("run: insert event: %v", err)
	}
	return n > 0, nil
}

func (r *RunRepo) EventsAfter(ctx context.Context, runID uuid.UUID, after int64, limit int) ([]run.Event, error) {
	lim := int64(limit)
	rows, err := r.q.RunEventsAfter(ctx, db.RunEventsAfterParams{RunID: runID, After: after, Lim: &lim})
	if err != nil {
		return nil, infraf("run: events: %v", err)
	}
	out := make([]run.Event, 0, len(rows))
	for _, row := range rows {
		e := run.Event{ID: row.ID, RunID: row.RunID, GrapheneID: row.GrapheneID, At: row.At, Kind: row.Kind, Title: row.Title, Subject: row.Subject, Status: row.Status, Error: row.Error, Attempt: int(row.Attempt)}
		_ = json.Unmarshal(row.Payload, &e.Payload) //nolint:errcheck // stored by us
		out = append(out, e)
	}
	return out, nil
}

func (r *RunRepo) SetFavorite(ctx context.Context, userID, tenantID uuid.UUID, kind string, targetID uuid.UUID, on bool) error {
	if on {
		if err := r.q.SetFavorite(ctx, db.SetFavoriteParams{UserID: userID, TenantID: tenantID, Kind: kind, TargetID: targetID}); err != nil {
			return infraf("favorite: set: %v", err)
		}
		return nil
	}
	if _, err := r.q.UnsetFavorite(ctx, db.UnsetFavoriteParams{UserID: userID, Kind: kind, TargetID: targetID}); err != nil {
		return infraf("favorite: unset: %v", err)
	}
	return nil
}

func (r *RunRepo) FavoritesOf(ctx context.Context, userID, tenantID uuid.UUID, kind string) (map[uuid.UUID]bool, error) {
	rows, err := r.q.FavoritesOf(ctx, db.FavoritesOfParams{UserID: userID, TenantID: tenantID, Kind: kind})
	if err != nil {
		return nil, infraf("favorite: list: %v", err)
	}
	out := map[uuid.UUID]bool{}
	for _, row := range rows {
		out[row.TargetID] = true
	}
	return out, nil
}

func runOf(row db.RunByIDRow) run.Run {
	x := run.Run{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name, Status: run.Status(row.Status), Phase: run.Phase(row.Phase), StatusReason: row.StatusReason,
		Trigger: run.Trigger(row.Trigger), SuiteRunID: row.SuiteRunID, CellID: row.CellID, ScheduleID: row.ScheduleID, ParentRunID: row.ParentRunID,
		TestID: row.TestID, TestName: row.TestName, AuthorID: row.AuthorID, RunSpec: row.RunSpec, Result: row.Result, LastEventID: row.LastEventID,
		RatingTenant: row.RatingTenant, RatingGlobal: row.RatingGlobal, KeepUntil: row.KeepUntil, StandKept: row.StandKept, Notes: row.Notes,
		GrapheneNamespace: row.GrapheneNamespace, GrapheneRunID: row.GrapheneRunID, PipelineRevision: row.PipelineRevision, TPS: row.Tps, DurationSeconds: row.DurationSeconds,
		CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt, Labels: map[string]string{},
	}
	_ = json.Unmarshal(row.Snapshot, &x.Snapshot)  //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.Summary, &x.Summary)    //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.RuntimeState, &x.State) //nolint:errcheck // stored by us
	_ = json.Unmarshal(row.Labels, &x.Labels)      //nolint:errcheck // stored by us
	if row.Keep != "" {
		x.Keep, _ = time.ParseDuration(row.Keep) //nolint:errcheck // stored by us
	}
	if len(row.Result) > 0 && string(row.Result) == "null" {
		x.Result = nil
	}
	return x
}
