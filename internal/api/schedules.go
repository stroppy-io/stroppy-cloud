package api

import (
	"context"
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/schedule"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/suite"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func fmtInt(i int) string { return strconv.Itoa(i) }

func scheduleOverridesFrom(o oas.OptLaunchOverrides) (schedule.Overrides, error) {
	s, err := suiteOverridesFrom(o)
	if err != nil {
		return schedule.Overrides{}, err
	}
	return schedule.Overrides{Name: s.Name, ProviderProfileID: s.ProviderProfileID, Sizes: s.Sizes, Keep: s.Keep, RatingTenant: s.RatingTenant, RatingGlobal: s.RatingGlobal, Labels: s.Labels, Notes: s.Notes}, nil
}

func runRefOf(r schedule.RunRef) oas.ScheduleRunRef {
	out := oas.ScheduleRunRef{Kind: oas.ScheduleRunRefKind(r.Kind), Status: oas.RunStatus(orDefault(r.Status, "failed")), At: r.At}
	if r.ID != nil {
		out.ID = *r.ID
	}
	if r.Name != "" {
		out.Name = oas.NewOptString(r.Name)
	}
	if r.Error != "" {
		out.Error = oas.NewOptString(r.Error)
	}
	return out
}

func scheduleOf(s schedule.Schedule) *oas.Schedule {
	out := &oas.Schedule{
		ID: s.ID, Name: s.Name, Target: oas.ScheduleTarget{Kind: oas.ScheduleTargetKind(s.TargetKind), ID: s.TargetID, Name: oas.NewOptString(s.TargetName)},
		Cron: s.Cron, Timezone: s.Timezone, Enabled: s.Enabled, NextRunAt: optTime(s.NextRunAt), CreatedAt: s.CreatedAt, UpdatedAt: oas.NewOptDateTime(s.UpdatedAt),
	}
	so := s.Overrides
	out.Overrides = suiteOverridesTo(suite.Overrides{Name: so.Name, ProviderProfileID: so.ProviderProfileID, Sizes: so.Sizes, Keep: so.Keep, RatingTenant: so.RatingTenant, RatingGlobal: so.RatingGlobal, Labels: so.Labels, Notes: so.Notes})
	if s.LastRun != nil {
		out.LastRun = oas.NewOptScheduleRunRef(runRefOf(*s.LastRun))
	}
	if s.AuthorID != nil {
		out.Author = oas.NewOptUserRef(oas.UserRef{ID: s.AuthorID.String()})
	}
	return out
}

// ListSchedules — of the tenant.
func (h *Handler) ListSchedules(ctx context.Context, params oas.ListSchedulesParams) (*oas.ListSchedulesOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	offset, err := cursorOffset(params.Cursor)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := schedule.ListQuery{Sort: string(params.Sort.Or(oas.ListSchedulesSortCreatedAt)), Desc: params.Order.Or(oas.OrderDesc) == oas.OrderDesc, Limit: limit + 1, Offset: offset}
	if v, ok := params.TargetKind.Get(); ok {
		q.TargetKind = string(v)
	}
	if v, ok := params.Enabled.Get(); ok {
		q.Enabled = &v
	}
	list, err := h.deps.Schedules.List(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.ListSchedulesOK{Data: make([]oas.Schedule, 0, len(list)), Meta: meta}
	for _, s := range list {
		out.Data = append(out.Data, *scheduleOf(s))
	}
	return out, nil
}

// CreateSchedule — store.
func (h *Handler) CreateSchedule(ctx context.Context, req *oas.ScheduleWrite, params oas.CreateScheduleParams) (*oas.Schedule, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	o, err := scheduleOverridesFrom(req.Overrides)
	if err != nil {
		return nil, err
	}
	w := schedule.Write{Name: req.Name, TargetKind: schedule.TargetKind(req.Target.Kind), TargetID: req.Target.ID, Cron: req.Cron, Timezone: req.Timezone.Or("UTC"), Overrides: o}
	if v, ok := req.Enabled.Get(); ok {
		w.Enabled = &v
	}
	s, err := h.deps.Schedules.Create(ctx, a, t.ID, w)
	if err != nil {
		return nil, err
	}
	return scheduleOf(s), nil
}

// GetSchedule — one.
func (h *Handler) GetSchedule(ctx context.Context, params oas.GetScheduleParams) (*oas.Schedule, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	s, err := h.deps.Schedules.Get(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return scheduleOf(s), nil
}

// PatchSchedule — partial update.
func (h *Handler) PatchSchedule(ctx context.Context, req *oas.SchedulePatch, params oas.PatchScheduleParams) (*oas.Schedule, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p := schedule.Patch{}
	if v, ok := req.Name.Get(); ok {
		p.Name = &v
	}
	if v, ok := req.Target.Get(); ok {
		k := schedule.TargetKind(v.Kind)
		id := v.ID
		p.TargetKind, p.TargetID = &k, &id
	}
	if v, ok := req.Cron.Get(); ok {
		p.Cron = &v
	}
	if v, ok := req.Timezone.Get(); ok {
		p.Timezone = &v
	}
	if v, ok := req.Enabled.Get(); ok {
		p.Enabled = &v
	}
	if req.Overrides.Set {
		o, err := scheduleOverridesFrom(req.Overrides)
		if err != nil {
			return nil, err
		}
		p.Overrides = &o
	}
	s, err := h.deps.Schedules.Update(ctx, a, t.ID, params.ID, p)
	if err != nil {
		return nil, err
	}
	return scheduleOf(s), nil
}

// DeleteSchedule — remove.
func (h *Handler) DeleteSchedule(ctx context.Context, params oas.DeleteScheduleParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Schedules.Delete(ctx, a, t.ID, params.ID)
}

// PauseSchedule — disable.
func (h *Handler) PauseSchedule(ctx context.Context, params oas.PauseScheduleParams) (*oas.Schedule, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	s, err := h.deps.Schedules.SetEnabled(ctx, a, t.ID, params.ID, false)
	if err != nil {
		return nil, err
	}
	return scheduleOf(s), nil
}

// ResumeSchedule — enable.
func (h *Handler) ResumeSchedule(ctx context.Context, params oas.ResumeScheduleParams) (*oas.Schedule, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	s, err := h.deps.Schedules.SetEnabled(ctx, a, t.ID, params.ID, true)
	if err != nil {
		return nil, err
	}
	return scheduleOf(s), nil
}

// RunScheduleNow — fire once.
func (h *Handler) RunScheduleNow(ctx context.Context, params oas.RunScheduleNowParams) (*oas.ScheduleRunRef, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	ref, err := h.deps.Schedules.RunNow(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	out := runRefOf(ref)
	return &out, nil
}

// ListScheduleHistory — what the schedule launched.
func (h *Handler) ListScheduleHistory(ctx context.Context, params oas.ListScheduleHistoryParams) (*oas.ListScheduleHistoryOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	offset, err := cursorOffset(params.Cursor)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	list, err := h.deps.Schedules.History(ctx, a, t.ID, params.ID, limit+1, offset)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.ListScheduleHistoryOK{Data: make([]oas.ScheduleRunRef, 0, len(list)), Meta: meta}
	for _, r := range list {
		out.Data = append(out.Data, runRefOf(r))
	}
	return out, nil
}
