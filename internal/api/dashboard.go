package api

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/rating"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/suite"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// GetTenantDashboard — the landing page: counts, recent activity, what is
// coming, best results, providers, limits.
func (h *Handler) GetTenantDashboard(ctx context.Context, params oas.GetTenantDashboardParams) (*oas.TenantDashboard, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	counts, err := h.deps.Runs.Counts(ctx, a, t.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.TenantDashboard{
		RunCounts: oas.TenantDashboardRunCounts{
			Total: oas.NewOptInt(counts.Total), Pending: oas.NewOptInt(counts.Pending), Running: oas.NewOptInt(counts.Running), Completed: oas.NewOptInt(counts.Completed),
			Failed: oas.NewOptInt(counts.Failed), Cancelled: oas.NewOptInt(counts.Cancelled), KeptStands: oas.NewOptInt(counts.KeptStands),
		},
		RecentRuns: []oas.Run{}, RecentSuiteRuns: []oas.SuiteRun{}, Upcoming: []oas.Schedule{}, TopResults: []oas.RatingEntry{}, Providers: []oas.ProviderProfile{},
		Pipelines: oas.TenantPipelines{Status: oas.TenantPipelinesStatusSynced},
	}
	if h.deps.Pipelines != nil {
		push := h.deps.Pipelines.TenantPipelines(ctx, t.GrapheneNamespace)
		out.Pipelines = oas.TenantPipelines{Status: oas.TenantPipelinesStatus(push.Status), Pushed: push.Pushed, Total: push.Total}
		if push.Error != "" {
			out.Pipelines.Error = oas.NewOptString(push.Error)
		}
	}
	if finished := counts.Completed + counts.Failed + counts.Cancelled; finished > 0 {
		out.SuccessRate = oas.NewOptFloat64(float64(counts.Completed) / float64(finished) * 100)
	}
	recent, err := h.deps.Runs.List(ctx, a, t.ID, run.ListQuery{Sort: "created_at", Desc: true, Limit: 5})
	if err != nil {
		return nil, err
	}
	favs := h.favoritesOf(ctx, a, t.ID)
	for _, r := range recent {
		f := favs[r.ID]
		out.RecentRuns = append(out.RecentRuns, *h.runOf(r, &f))
	}
	suiteRuns, err := h.deps.Suites.Runs(ctx, a, t.ID, suite.ListQuery{Sort: "started_at", Desc: true, Limit: 5})
	if err != nil {
		return nil, err
	}
	for _, r := range suiteRuns {
		out.RecentSuiteRuns = append(out.RecentSuiteRuns, *h.suiteRunOf(r))
	}
	upcoming, err := h.deps.Schedules.Upcoming(ctx, a, t.ID, 5)
	if err != nil {
		return nil, err
	}
	for _, s := range upcoming {
		out.Upcoming = append(out.Upcoming, *scheduleOf(s))
	}
	if top, err := h.deps.Rating.Tenant(ctx, a, t.ID, rating.Query{Metric: "tps", Limit: 5}); err == nil {
		out.TopResults = ratingPageOf(top, 0, 5, false).Data
	}
	profiles, err := h.deps.Providers.List(ctx, a, t.ID)
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		out.Providers = append(out.Providers, profileOf(p))
	}
	if l, err := h.deps.Settings.Limits(ctx, a, t.ID); err == nil {
		src := oas.TenantLimitsSourcePlatformDefault
		if l.Overridden {
			src = oas.TenantLimitsSourceTenantOverride
		}
		out.Limits = oas.NewOptTenantLimits(oas.TenantLimits{MaxConcurrentRuns: l.MaxConcurrentRuns, MaxMachinesPerRun: l.MaxMachinesPerRun, MaxSize: oas.Size(l.MaxSize), MaxKeep: l.MaxKeep.String(), RunRetentionMaxDays: l.RunRetentionMaxDays, Source: oas.NewOptTenantLimitsSource(src)})
	}
	return out, nil
}
