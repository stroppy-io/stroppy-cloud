// Package rating ranks opted-in completed runs by a headline metric
// (§16.8): per tenant and public (global). A number is comparable only
// within one scenario on the same hardware, so every row carries both and
// the page lists the ones that have runs, to filter by.
package rating

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// Query filters a rating page.
type Query struct {
	Metric          string
	Kinds           []string
	Versions        []string
	Providers       []string
	StroppyVersions []string
	Scenarios       []string
	Hardware        []string
	Period          string // 7d|30d|90d|1y|all
	Limit           int
	Offset          int
}

// Entry is one ranked run.
type Entry struct {
	Rank       int
	Value      float64
	Run        run.Run
	Scenario   string
	Hardware   string
	TenantName string
	ShareToken string
}

// Page is a rating page.
type Page struct {
	Metric    catalog.Metric
	Scenarios []string
	Hardware  []string
	Entries   []Entry
}

// Shares resolves the public share a global-rating row links to (created
// for the run on first appearance if it was never shared).
type Shares interface {
	RatingTokenOf(ctx context.Context, r run.Run) (string, bool)
}

// Access resolves the caller.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
}

// Service is the rating queries.
type Service struct {
	runs    run.Repository
	access  Access
	catalog *catalog.Catalog
	shares  Shares
}

// NewService wires the use case.
func NewService(runs run.Repository, access Access, cat *catalog.Catalog, shares Shares) *Service {
	return &Service{runs: runs, access: access, catalog: cat, shares: shares}
}

// Tenant is the tenant's own rating (runs with rating.tenant).
func (s *Service) Tenant(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q Query) (Page, error) {
	if _, _, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return Page{}, err
	}
	return s.page(ctx, tenantID.String(), q)
}

// Public is the global rating (runs with rating.global); tenant names are
// the tenants' public names.
func (s *Service) Public(ctx context.Context, q Query) (Page, error) {
	return s.page(ctx, "", q)
}

func (s *Service) page(ctx context.Context, tenantID string, q Query) (Page, error) {
	metric := q.Metric
	if metric == "" {
		metric = "tps"
	}
	var def catalog.Metric
	found := false
	for _, m := range s.catalog.MetricsFor("") {
		if m.Key == metric {
			def, found = m, true
		}
	}
	if !found || !def.RatingEligible {
		return Page{}, errs.Invalid("metric "+metric+" is not rating eligible").Because("metric_not_rating_eligible", "metric", metric)
	}
	rq := run.RatingQuery{
		Metric: metric, HigherIsBetter: def.HigherIsBetter, TenantID: tenantID, Kinds: q.Kinds, Versions: q.Versions, Providers: q.Providers,
		StroppyVersions: q.StroppyVersions, Scenarios: q.Scenarios, Hardware: q.Hardware, Since: since(q.Period), Limit: q.Limit, Offset: q.Offset,
	}
	rows, err := s.runs.Rating(ctx, rq)
	if err != nil {
		return Page{}, err
	}
	facets, err := s.runs.RatingFacets(ctx, tenantID)
	if err != nil {
		return Page{}, err
	}
	out := Page{
		Metric: def, Scenarios: distinct(facets, func(f run.RatingFacet) string { return f.Scenario }),
		Hardware: distinct(facets, func(f run.RatingFacet) string { return f.Hardware }), Entries: make([]Entry, 0, len(rows)),
	}
	for i, r := range rows {
		e := Entry{Rank: q.Offset + i + 1, Value: r.Value, Run: r.Run, Scenario: r.Scenario, Hardware: r.Hardware, TenantName: r.TenantName}
		if tenantID == "" {
			e.TenantName = r.TenantPublicName
			if s.shares != nil {
				if tok, ok := s.shares.RatingTokenOf(ctx, r.Run); ok {
					e.ShareToken = tok
				}
			}
		}
		out.Entries = append(out.Entries, e)
	}
	return out, nil
}

// distinct is the non-empty values of one facet column, in first-seen order
// (the query sorts them).
func distinct(facets []run.RatingFacet, of func(run.RatingFacet) string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, f := range facets {
		if v := of(f); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func since(period string) time.Time {
	now := time.Now().UTC()
	switch period {
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	case "90d":
		return now.AddDate(0, 0, -90)
	case "1y":
		return now.AddDate(-1, 0, 0)
	}
	return time.Time{}
}
