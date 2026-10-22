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
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
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

func (r *RunRepo) Admit(ctx context.Context, tenantID uuid.UUID, fn func(context.Context) error) error {
	return r.tx.Do(ctx, func(ctx context.Context) error {
		tenant, err := r.q.LockTenantLifecycle(ctx, tenantID)
		if err != nil {
			return err
		}
		if tenant.Retiring {
			return errs.Conflict("tenant is being deleted").Because("tenant_is_being_deleted")
		}
		return fn(ctx)
	})
}

func (r *RunRepo) Insert(ctx context.Context, x run.Run, idempotencyKey string) error {
	return r.tx.Do(ctx, func(ctx context.Context) error {
		tenant, err := r.q.LockTenantLifecycle(ctx, x.TenantID)
		if err != nil {
			return err
		}
		if tenant.Retiring {
			return errs.Conflict("tenant is being deleted").Because("tenant_is_being_deleted")
		}
		profile, err := r.q.LockProviderLifecycle(ctx, x.Snapshot.ProviderProfile.ID)
		if err != nil {
			return errs.Conflict("provider is unavailable").Because("provider_is_unavailable")
		}
		if profile.Status != "ready" {
			return errs.Conflict("provider is not ready").Because("provider_is_not_ready")
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
		CreatedAt: createdAt(x.CreatedAt),
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a run with this idempotency key exists").Because("run_idempotency_conflict")
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
	durMin, durMax := q.DurationMin.Seconds(), q.DurationMax.Seconds()
	rows, err := r.q.RunsOfTenant(ctx, db.RunsOfTenantParams{
		TenantID: tenantID, Search: q.Search, AuthorID: q.AuthorID, Statuses: q.Statuses, Kinds: q.Kinds, Profiles: q.Profiles, Triggers: q.Triggers,
		TestID: q.TestID, SuiteRunID: q.SuiteRunID, Standalone: q.Standalone, Labels: tagsFilter(q.Labels),
		StartedAfter: &q.StartedAfter, StartedBefore: &q.StartedBefore, FinishedAfter: &q.FinishedAfter, FinishedBefore: &q.FinishedBefore,
		DurationMin: durMin, DurationMax: durMax, StandKeptOnly: q.StandKeptOnly, FavoritesOf: q.FavoritesOf,
		ViewerID: q.ViewerID, SortKey: q.Sort, Desc: q.Desc, Lim: int64(limit), Off: int64(q.Offset),
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
		if row.Value == nil { // a JSON null under a present key: no facet value
			continue
		}
		if len(out) == 0 || out[len(out)-1].Field != row.Field {
			out = append(out, run.Facet{Field: row.Field, Values: []run.FacetValue{}})
		}
		out[len(out)-1].Values = append(out[len(out)-1].Values, run.FacetValue{Value: *row.Value, Count: int(row.Count)})
	}
	return out, nil
}

func (r *RunRepo) TestRunStats(ctx context.Context, testIDs []uuid.UUID) (map[uuid.UUID]run.TestRunStat, error) {
	rows, err := r.q.TestRunStats(ctx, db.TestRunStatsParams{TestIds: testIDs})
	if err != nil {
		return nil, infraf("run: test stats: %v", err)
	}
	out := make(map[uuid.UUID]run.TestRunStat, len(rows))
	for _, row := range rows {
		if row.TestID == nil {
			continue
		}
		out[*row.TestID] = run.TestRunStat{Last: run.Ref{ID: row.ID, Name: row.Name}, Status: run.Status(row.Status), StartedAt: row.StartedAt, Count: int(row.RunCount)}
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
		Scenarios: q.Scenarios, Hardware: q.Hardware, Since: &q.Since, HigherIsBetter: q.HigherIsBetter, Lim: int64(limit), Off: int64(q.Offset),
	})
	if err != nil {
		return nil, infraf("run: rating: %v", err)
	}
	out := make([]run.RatingRow, 0, len(rows))
	for _, row := range rows {
		x := run.Run{ID: row.ID, TenantID: row.TenantID, Name: row.Name, AuthorID: row.AuthorID, FinishedAt: row.FinishedAt, TPS: row.Tps}
		_ = json.Unmarshal(row.Summary, &x.Summary) //nolint:errcheck // stored by us
		item := run.RatingRow{Run: x, TenantName: row.TenantName, Scenario: row.Scenario, Hardware: row.Hardware}
		if row.TenantPublicName != nil {
			item.TenantPublicName = *row.TenantPublicName
		}
		if row.Value != nil {
			item.Value = *row.Value
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *RunRepo) RatingFacets(ctx context.Context, tenantID string) ([]run.RatingFacet, error) {
	rows, err := r.q.RatingFacets(ctx, tenantID)
	if err != nil {
		return nil, infraf("run: rating facets: %v", err)
	}
	out := make([]run.RatingFacet, 0, len(rows))
	for _, row := range rows {
		out = append(out, run.RatingFacet{Scenario: row.Scenario, Hardware: row.Hardware})
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

func (r *RunRepo) HasLive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	row, err := r.q.TenantHasLiveRuns(ctx, tenantID)
	if err != nil {
		return false, infraf("run: has live: %v", err)
	}
	return row.Live, nil
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
	value := string(status)
	if err := r.q.SetRunStatus(ctx, db.SetRunStatusParams{ID: id, Status: value, Phase: string(phase), StatusReason: reason, StartedAt: startedAt, FinishedAt: finishedAt}); err != nil {
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

func (r *RunRepo) WithoutQPSSeries(ctx context.Context, limit int) ([]run.Run, error) {
	rows, err := r.q.RunsWithoutQpsSeries(ctx, int64(limit))
	if err != nil {
		return nil, infraf("run: without qps series: %v", err)
	}
	out := make([]run.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runOf(db.RunByIDRow(row)))
	}
	return out, nil
}

func (r *RunRepo) SetQPSSeries(ctx context.Context, id uuid.UUID, series []run.Point) error {
	if series == nil {
		series = []run.Point{}
	}
	raw, _ := json.Marshal(series) //nolint:errcheck // plain struct slice
	if err := r.q.SetRunQpsSeries(ctx, db.SetRunQpsSeriesParams{ID: id, Series: raw}); err != nil {
		return infraf("run: set qps series: %v", err)
	}
	return nil
}

func (r *RunRepo) SetKeep(ctx context.Context, id uuid.UUID, kept bool, until *time.Time) error {
	if err := r.q.SetRunKeep(ctx, db.SetRunKeepParams{ID: id, StandKept: kept, KeepUntil: until}); err != nil {
		return infraf("run: set keep: %v", err)
	}
	return nil
}

func (r *RunRepo) WithoutTopology(ctx context.Context, limit int) ([]run.Run, error) {
	rows, err := r.q.RunsWithoutTopology(ctx, int64(limit))
	if err != nil {
		return nil, infraf("run: without topology: %v", err)
	}
	out := make([]run.Run, 0, len(rows))
	for _, row := range rows {
		x := run.Run{ID: row.ID}
		_ = json.Unmarshal(row.Snapshot, &x.Snapshot) //nolint:errcheck // stored by us
		out = append(out, x)
	}
	return out, nil
}

func (r *RunRepo) SetTopology(ctx context.Context, id uuid.UUID, s topology.Shape) error {
	raw, _ := json.Marshal(s) //nolint:errcheck // plain struct
	if err := r.q.SetRunTopology(ctx, db.SetRunTopologyParams{Topology: raw, ID: id}); err != nil {
		return infraf("run: set topology: %v", err)
	}
	return nil
}

func (r *RunRepo) Queued(ctx context.Context) ([]run.Run, error) {
	rows, err := r.q.QueuedRuns(ctx)
	if err != nil {
		return nil, infraf("run: queued: %v", err)
	}
	out := make([]run.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runOf(db.RunByIDRow(row)))
	}
	return out, nil
}

func (r *RunRepo) QueuedOf(ctx context.Context, tenantID uuid.UUID) ([]run.Run, error) {
	rows, err := r.q.QueuedRunsOfTenant(ctx, tenantID)
	if err != nil {
		return nil, infraf("run: queued of tenant: %v", err)
	}
	out := make([]run.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runOf(db.RunByIDRow(row)))
	}
	return out, nil
}

// createdAt keeps the caller's creation time; a zero one is now.
func createdAt(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

func (r *RunRepo) AdmitQueued(ctx context.Context, id uuid.UUID) (bool, error) {
	n, err := r.q.AdmitRun(ctx, id)
	if err != nil {
		return false, infraf("run: admit: %v", err)
	}
	return n > 0, nil
}

func (r *RunRepo) SetQueue(ctx context.Context, id uuid.UUID, q run.Queue) error {
	raw, err := json.Marshal(q)
	if err != nil {
		return err
	}
	if err := r.q.SetRunQueue(ctx, db.SetRunQueueParams{Queue: raw, ID: id}); err != nil {
		return infraf("run: queue: %v", err)
	}
	return nil
}

func (r *RunRepo) CancelQueued(ctx context.Context, id uuid.UUID, reason string) (bool, error) {
	n, err := r.q.CancelQueuedRun(ctx, db.CancelQueuedRunParams{StatusReason: reason, ID: id})
	if err != nil {
		return false, infraf("run: cancel queued: %v", err)
	}
	return n > 0, nil
}

func (r *RunRepo) Starting(ctx context.Context, tenantID uuid.UUID) ([]run.Starting, error) {
	rows, err := r.q.StartingRuns(ctx, tenantID)
	if err != nil {
		return nil, infraf("run: starting: %v", err)
	}
	out := make([]run.Starting, 0, len(rows))
	for _, row := range rows {
		st := run.Starting{Spec: row.RunSpec}
		if row.ProfileID != nil {
			st.ProfileID = *row.ProfileID
		}
		out = append(out, st)
	}
	return out, nil
}

func (r *RunRepo) NetworkRuns(ctx context.Context, tenantID, profileID uuid.UUID, since time.Time) ([]run.NetworkRun, error) {
	rows, err := r.q.ProfileNetworkRuns(ctx, db.ProfileNetworkRunsParams{TenantID: tenantID, ProfileID: profileID.String(), Since: &since})
	if err != nil {
		return nil, infraf("run: network runs: %v", err)
	}
	out := make([]run.NetworkRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, run.NetworkRun{Spec: row.RunSpec, Holds: row.Holds != nil && *row.Holds})
	}
	return out, nil
}

func (r *RunRepo) SetQueuedSpec(ctx context.Context, id uuid.UUID, spec json.RawMessage) error {
	n, err := r.q.SetQueuedRunSpec(ctx, db.SetQueuedRunSpecParams{RunSpec: spec, ID: id})
	if err != nil {
		return infraf("run: queued spec: %v", err)
	}
	if n == 0 {
		return infraf("run: queued spec: run %s is not queued", id)
	}
	return nil
}

func (r *RunRepo) LiveCells(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.q.LiveCellCounts(ctx, tenantID)
	if err != nil {
		return nil, infraf("run: live cells: %v", err)
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		if row.SuiteRunID != nil {
			out[*row.SuiteRunID] = int(row.N)
		}
	}
	return out, nil
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
	rows, err := r.q.RunEventsAfter(ctx, db.RunEventsAfterParams{RunID: runID, After: after, Lim: lim})
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
		AdmittedAt: row.AdmittedAt,
	}
	_ = json.Unmarshal(row.Queue, &x.Queue)        //nolint:errcheck // stored by us
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
