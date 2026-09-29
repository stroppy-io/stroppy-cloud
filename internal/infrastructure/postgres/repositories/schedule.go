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
	"github.com/stroppy-io/stroppy-cloud/internal/domain/schedule"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// ScheduleRepo stores schedules and their firing history.
type ScheduleRepo struct{ q *db.Queries }

// NewScheduleRepo builds the repository.
func NewScheduleRepo(database tx.DB) *ScheduleRepo { return &ScheduleRepo{q: db.New(database)} }

func (r *ScheduleRepo) Insert(ctx context.Context, s schedule.Schedule) error {
	ov, _ := json.Marshal(s.Overrides) //nolint:errcheck // struct
	err := r.q.InsertSchedule(ctx, db.InsertScheduleParams{
		ID: s.ID, TenantID: s.TenantID, Name: s.Name, TargetKind: string(s.TargetKind), TargetID: s.TargetID, TargetName: s.TargetName,
		Cron: s.Cron, Timezone: s.Timezone, Enabled: s.Enabled, Overrides: ov, NextRunAt: s.NextRunAt, AuthorID: s.AuthorID,
	})
	if err != nil {
		return infraf("schedule: insert: %v", err)
	}
	return nil
}

func (r *ScheduleRepo) ByID(ctx context.Context, id uuid.UUID) (schedule.Schedule, error) {
	row, err := r.q.ScheduleByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schedule.Schedule{}, errs.NotFound("schedule")
		}
		return schedule.Schedule{}, infraf("schedule: by id: %v", err)
	}
	return scheduleOf(row), nil
}

func (r *ScheduleRepo) List(ctx context.Context, tenantID uuid.UUID, q schedule.ListQuery) ([]schedule.Schedule, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	onlyEnabled, onlyDisabled := false, false
	if q.Enabled != nil {
		onlyEnabled, onlyDisabled = *q.Enabled, !*q.Enabled
	}
	rows, err := r.q.SchedulesOfTenant(ctx, db.SchedulesOfTenantParams{TenantID: tenantID, TargetKind: q.TargetKind, OnlyEnabled: onlyEnabled, OnlyDisabled: onlyDisabled, SortKey: q.Sort, Desc: q.Desc, Lim: int64(limit), Off: int64(q.Offset)})
	if err != nil {
		return nil, infraf("schedule: list: %v", err)
	}
	out := make([]schedule.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleOf(db.ScheduleByIDRow(row)))
	}
	return out, nil
}

func (r *ScheduleRepo) Upcoming(ctx context.Context, tenantID uuid.UUID, limit int) ([]schedule.Schedule, error) {
	rows, err := r.q.UpcomingSchedules(ctx, db.UpcomingSchedulesParams{TenantID: tenantID, Lim: int64(limit)})
	if err != nil {
		return nil, infraf("schedule: upcoming: %v", err)
	}
	out := make([]schedule.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleOf(db.ScheduleByIDRow(row)))
	}
	return out, nil
}

func (r *ScheduleRepo) Due(ctx context.Context, now time.Time) ([]schedule.Schedule, error) {
	rows, err := r.q.DueSchedules(ctx, &now)
	if err != nil {
		return nil, infraf("schedule: due: %v", err)
	}
	out := make([]schedule.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleOf(db.ScheduleByIDRow(row)))
	}
	return out, nil
}

func (r *ScheduleRepo) Update(ctx context.Context, id uuid.UUID, p schedule.Patch, targetName string, nextRunAt *time.Time) error {
	params := db.UpdateScheduleParams{ID: id, Name: p.Name, TargetID: p.TargetID, Cron: p.Cron, Timezone: p.Timezone, Enabled: p.Enabled, NextRunAt: nextRunAt}
	if p.TargetKind != nil {
		k := string(*p.TargetKind)
		params.TargetKind = &k
	}
	if p.TargetKind != nil || p.TargetID != nil {
		params.TargetName = &targetName
	}
	if p.Overrides != nil {
		params.Overrides, _ = json.Marshal(p.Overrides) //nolint:errcheck // struct
	}
	n, err := r.q.UpdateSchedule(ctx, params)
	if err != nil {
		return infraf("schedule: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("schedule")
	}
	return nil
}

func (r *ScheduleRepo) SetFired(ctx context.Context, id uuid.UUID, nextRunAt *time.Time, last schedule.RunRef) error {
	raw, _ := json.Marshal(last) //nolint:errcheck // struct
	if err := r.q.SetScheduleFired(ctx, db.SetScheduleFiredParams{ID: id, NextRunAt: nextRunAt, LastRun: raw}); err != nil {
		return infraf("schedule: set fired: %v", err)
	}
	return nil
}

func (r *ScheduleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteSchedule(ctx, id)
	if err != nil {
		return infraf("schedule: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("schedule")
	}
	return nil
}

func (r *ScheduleRepo) AddHistory(ctx context.Context, id uuid.UUID, ref schedule.RunRef) error {
	if err := r.q.InsertScheduleHistory(ctx, db.InsertScheduleHistoryParams{ScheduleID: id, Kind: ref.Kind, RefID: ref.ID, Name: ref.Name, Status: ref.Status, Error: ref.Error}); err != nil {
		return infraf("schedule: history: %v", err)
	}
	return nil
}

func (r *ScheduleRepo) History(ctx context.Context, id uuid.UUID, limit, offset int) ([]schedule.RunRef, error) {
	rows, err := r.q.ScheduleHistory(ctx, db.ScheduleHistoryParams{ScheduleID: id, Lim: int64(limit), Off: int64(offset)})
	if err != nil {
		return nil, infraf("schedule: history: %v", err)
	}
	out := make([]schedule.RunRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, schedule.RunRef{Kind: row.Kind, ID: row.RefID, Name: row.Name, Status: row.Status, Error: row.Error, At: row.At})
	}
	return out, nil
}

func scheduleOf(row db.ScheduleByIDRow) schedule.Schedule {
	s := schedule.Schedule{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name, TargetKind: schedule.TargetKind(row.TargetKind), TargetID: row.TargetID, TargetName: row.TargetName,
		Cron: row.Cron, Timezone: row.Timezone, Enabled: row.Enabled, NextRunAt: row.NextRunAt, AuthorID: row.AuthorID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	_ = json.Unmarshal(row.Overrides, &s.Overrides) //nolint:errcheck // stored by us
	if len(row.LastRun) > 0 && string(row.LastRun) != "null" {
		var last schedule.RunRef
		if json.Unmarshal(row.LastRun, &last) == nil {
			s.LastRun = &last
		}
	}
	return s
}
