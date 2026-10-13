package api

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/rating"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func ratingQueryOf(metric oas.OptString, kinds []oas.DatabaseKind, versions []string, providers []oas.ProviderKind, stroppy, scenarios, hardware []string, period oas.OptRatingPeriod, cursor oas.OptString, limit oas.OptInt) (rating.Query, int, error) {
	offset, err := cursorOffset(cursor)
	if err != nil {
		return rating.Query{}, 0, err
	}
	lim := limit.Or(50)
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	q := rating.Query{Metric: metric.Or("tps"), Versions: versions, StroppyVersions: stroppy, Scenarios: scenarios, Hardware: hardware, Period: string(period.Or(oas.RatingPeriodAll)), Limit: lim + 1, Offset: offset}
	for _, k := range kinds {
		q.Kinds = append(q.Kinds, string(k))
	}
	for _, p := range providers {
		q.Providers = append(q.Providers, string(p))
	}
	return q, lim, nil
}

// hardwareOf is the API shape of a rating hardware key.
func hardwareOf(key string) oas.HardwareClass {
	out := oas.HardwareClass{Key: key, Machines: []oas.HardwareClassMachinesItem{}}
	for _, g := range run.ParseHardware(key) {
		out.Machines = append(out.Machines, oas.HardwareClassMachinesItem{Count: g.Count, CPU: g.CPU, MemoryGB: g.MemoryGB})
	}
	return out
}

func ratingPageOf(p rating.Page, offset, limit int, public bool) *oas.RatingPage {
	entries, meta := page(p.Entries, offset, limit)
	out := &oas.RatingPage{Metric: metricOf(p.Metric), Scenarios: p.Scenarios, Hardware: make([]oas.HardwareClass, 0, len(p.Hardware)), Data: make([]oas.RatingEntry, 0, len(entries)), Meta: meta}
	if out.Scenarios == nil {
		out.Scenarios = []string{}
	}
	for _, key := range p.Hardware {
		out.Hardware = append(out.Hardware, hardwareOf(key))
	}
	for _, e := range entries {
		s := e.Run.Summary
		item := oas.RatingEntry{Rank: e.Rank, Value: e.Value, Unit: p.Metric.Unit, DbKind: oas.DatabaseKind(s.DBKind), Scenario: e.Scenario, Hardware: hardwareOf(e.Hardware), RunID: oas.NewOptUUID(e.Run.ID)}
		if e.Run.FinishedAt != nil {
			item.RunAt = *e.Run.FinishedAt
		}
		if s.DBVersion != "" {
			item.DbVersion = oas.NewOptString(s.DBVersion)
		}
		if s.TopologyLabel != "" {
			item.TopologyLabel = oas.NewOptString(s.TopologyLabel)
		}
		if s.NodeCount > 0 {
			item.NodeCount = oas.NewOptInt(s.NodeCount)
		}
		if len(s.Sizes) > 0 {
			sizes := oas.RoleSizes{}
			for role, size := range s.Sizes {
				sizes[role] = oas.RoleSizesItem{Size: oas.Size(size)}
			}
			item.Sizes = oas.NewOptRoleSizes(sizes)
		}
		if s.WorkloadName != "" {
			item.WorkloadName = oas.NewOptString(s.WorkloadName)
		}
		if s.StroppyVersion != "" {
			item.StroppyVersion = oas.NewOptString(s.StroppyVersion)
		}
		if s.ProviderKind != "" {
			item.ProviderKind = oas.NewOptProviderKind(oas.ProviderKind(s.ProviderKind))
		}
		if public {
			if e.TenantName != "" {
				item.TenantName = oas.NewOptString(e.TenantName)
			}
			if e.ShareToken != "" {
				item.ShareToken = oas.NewOptString(e.ShareToken)
			}
		} else if e.Run.AuthorID != nil {
			item.Author = oas.NewOptUserRef(oas.UserRef{ID: e.Run.AuthorID.String()})
		}
		out.Data = append(out.Data, item)
	}
	return out
}

// GetTenantRating — the tenant's own leaderboard.
func (h *Handler) GetTenantRating(ctx context.Context, params oas.GetTenantRatingParams) (*oas.RatingPage, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	q, limit, err := ratingQueryOf(params.Metric, params.Kind, params.Version, params.Provider, params.StroppyVersion, params.Scenario, params.Hardware, params.Period, params.Cursor, params.Limit)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Rating.Tenant(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	return ratingPageOf(p, q.Offset, limit, false), nil
}

// GetPublicRating — the global leaderboard (opt-in runs only).
func (h *Handler) GetPublicRating(ctx context.Context, params oas.GetPublicRatingParams) (*oas.RatingPage, error) {
	if !h.publicConfig(ctx).PublicRating {
		return nil, errs.NotFound("public rating")
	}
	q, limit, err := ratingQueryOf(params.Metric, params.Kind, params.Version, params.Provider, params.StroppyVersion, params.Scenario, params.Hardware, params.Period, params.Cursor, params.Limit)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Rating.Public(ctx, q)
	if err != nil {
		return nil, err
	}
	return ratingPageOf(p, q.Offset, limit, true), nil
}
